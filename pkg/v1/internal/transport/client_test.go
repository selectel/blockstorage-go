package transport

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_Scope(t *testing.T) {
	t.Run("UsesProvidedScope", func(t *testing.T) {
		client, err := NewClient(testEndpoint, testTokenID, "", nil)
		require.NoError(t, err)

		require.Equal(t, testEndpoint, client.endpoint.String())
		require.Equal(t, testTokenID, client.tokenID)
		require.NotNil(t, client.httpClient)
	})

	t.Run("DefaultUserAgent", func(t *testing.T) {
		client, err := NewClient(testEndpoint, testTokenID, "", nil)
		require.NoError(t, err)

		require.Equal(t, appName+"/"+unknownModuleVersion, client.userAgent)
	})

	t.Run("UserAgentPrefix", func(t *testing.T) {
		client, err := NewClient(testEndpoint, testTokenID, "terraform-provider-selectel/v7.0.0", nil)
		require.NoError(t, err)

		require.Equal(t, "terraform-provider-selectel/v7.0.0 "+appName+"/"+unknownModuleVersion, client.userAgent)
	})

	t.Run("StandardClientRefusesRedirects", func(t *testing.T) {
		httpClient := &http.Client{Timeout: time.Second}

		client, err := NewClient(testEndpoint, testTokenID, "", httpClient)
		require.NoError(t, err)

		copied, ok := client.httpClient.(*http.Client)
		require.True(t, ok)
		require.NotSame(t, httpClient, copied)
		require.Equal(t, time.Second, copied.Timeout)
		require.Nil(t, httpClient.CheckRedirect)
		require.ErrorIs(t, copied.CheckRedirect(nil, nil), http.ErrUseLastResponse)
	})

	t.Run("OwnImplementationIsUsedAsIs", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: "{}"}}}

		client, err := NewClient(testEndpoint, testTokenID, "", httpClient)
		require.NoError(t, err)

		require.Same(t, httpClient, client.httpClient)
	})

	t.Run("RejectsMissingEndpoint", func(t *testing.T) {
		client, err := NewClient("", testTokenID, "", nil)

		require.Nil(t, client)
		require.True(t, IsKind(err, KindInvalidRequest), "unexpected error: %v", err)
		require.Contains(t, err.Error(), "endpoint")
	})

	t.Run("RejectsInvalidEndpoint", func(t *testing.T) {
		client, err := NewClient("api.example.com/volume/v3/project", testTokenID, "", nil)

		require.Nil(t, client)
		require.True(t, IsKind(err, KindInvalidRequest), "unexpected error: %v", err)
		require.Contains(t, err.Error(), "absolute HTTP or HTTPS URL")
	})

	t.Run("RejectsMissingToken", func(t *testing.T) {
		client, err := NewClient(testEndpoint, "  ", "", nil)

		require.Nil(t, client)
		require.True(t, IsKind(err, KindInvalidRequest), "unexpected error: %v", err)
		require.Contains(t, err.Error(), "token")
	})
}

func TestClient_Request(t *testing.T) {
	t.Run("SendsScopeAndHeaders", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: ""}}}
		client := testClient(httpClient, "terraform-provider-selectel/v7.0.0")

		response, err := DoRequest(
			t.Context(), client, http.MethodPost, "volumes", http.StatusAccepted,
			map[string]string{"name": "disk"}, nil,
		)
		require.NoError(t, err)
		require.Equal(t, http.StatusAccepted, response.StatusCode)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, testEndpoint+"/volumes", request.URL.String())
		require.Equal(t, testTokenID, request.Header.Get(headerAuthToken))
		require.Equal(t, client.userAgent, request.Header.Get(headerUserAgent))
		require.Equal(t, contentTypeJSON, request.Header.Get(headerAccept))
		require.Equal(t, contentTypeJSON, request.Header.Get(headerContentType))
		require.Empty(t, request.Header.Get(headerAPIVersion))
		require.JSONEq(t, `{"name":"disk"}`, httpClient.lastBody(t))
	})

	t.Run("SendsMicroversion", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: ""}}}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodPost, "volumes", http.StatusOK,
			nil, nil, WithMicroversion("3.47"),
		)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, apiVersionService+" 3.47", request.Header.Get(headerAPIVersion))
		require.Empty(t, request.Header.Get(headerContentType))
	})

	t.Run("SendsQuery", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: ""}}}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes/detail", http.StatusOK,
			nil, nil, WithQuery(url.Values{"limit": []string{"100"}}),
		)
		require.NoError(t, err)

		require.Equal(t, "limit=100", httpClient.lastRequest(t).URL.RawQuery)
	})

	t.Run("DecodesResult", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: `{"volume": {"id": "0e1c2b3a", "status": "creating"}}`},
		}}

		var result struct {
			Volume struct {
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"volume"`
		}

		response, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes/0e1c2b3a", http.StatusOK,
			nil, &result, WithResponseEnvelope("volume"),
		)
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "0e1c2b3a", result.Volume.ID)
		require.Equal(t, "creating", result.Volume.Status)
	})

	t.Run("UndecodableResult", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: "not a json body"}}}

		var result map[string]any

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, &result,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindUnexpected), "unexpected error: %v", err)
	})

	t.Run("IgnoresUnreadBodyWithoutResult", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusAccepted, bodyErr: errors.New("connection reset by peer")},
		}}

		result, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodDelete, "volumes/0e1c2b3a", http.StatusAccepted,
			nil, nil,
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusAccepted, result.StatusCode)
	})
}

func TestClient_ResponseEnvelope(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{name: "EmptyBody", body: ""},
		{name: "NullBody", body: "null"},
		{name: "EmptyObject", body: "{}"},
		{name: "MissingEnvelope", body: `{"other": {}}`},
		{name: "NullEnvelope", body: `{"volume": null}`},
		{name: "EmptyEnvelope", body: `{"volume": {}}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: testCase.body}}}
			var result map[string]any

			response, err := DoRequest(
				t.Context(), testClient(httpClient), http.MethodGet, "volumes/0e1c2b3a", http.StatusOK,
				nil, &result, WithResponseEnvelope("volume"),
			)

			require.Equal(t, http.StatusOK, response.StatusCode)
			require.True(t, IsKind(err, KindUnexpected), "unexpected error: %v", err)
		})
	}
}

func TestClient_URLResolution(t *testing.T) {
	t.Run("AbsoluteURLKeepsPathAndQuery", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: ""}}}

		_, err := DoRequest(t.Context(), testClient(httpClient), http.MethodGet,
			testEndpoint+"/volumes/detail?marker=0e1c2b3a&limit=2", http.StatusOK, nil, nil)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, "/volume/v3/"+testProjectID+"/volumes/detail", request.URL.Path)
		require.Equal(t, "marker=0e1c2b3a&limit=2", request.URL.RawQuery)
	})

	t.Run("ForeignAbsoluteURLStaysOnTheEndpoint", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: ""}}}

		_, err := DoRequest(t.Context(), testClient(httpClient), http.MethodGet,
			"https://another.example.com/volume/v3/"+testProjectID+"/volumes/detail?marker=vol-1",
			http.StatusOK, nil, nil)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, "api.example.com", request.URL.Host)
		require.Equal(t, "/volume/v3/"+testProjectID+"/volumes/detail", request.URL.Path)
		require.Equal(t, "marker=vol-1", request.URL.RawQuery)
	})
}

func TestClient_NoHiddenRetry(t *testing.T) {
	t.Run("ServerError", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusInternalServerError, body: apiFaultJSON}}}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, nil,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindServerError))
		require.Len(t, httpClient.requests, 1, "the SDK must not retry the request")
	})

	t.Run("TransportFailure", func(t *testing.T) {
		httpClient := &testHTTPClient{err: errors.New("connection reset by peer")}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, nil,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindTransport), "unexpected error: %v", err)
		require.Len(t, httpClient.requests, 1, "the SDK must not retry the request")
	})

	t.Run("RedirectIsNotFollowed", func(t *testing.T) {
		foreign := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{"volume": {"id": "planted"}}`))
		}))
		defer foreign.Close()

		reached := false
		origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			reached = true
			writer.Header().Set("Location", foreign.URL+"/volumes")
			writer.WriteHeader(http.StatusTemporaryRedirect)
		}))
		defer origin.Close()

		client, clientErr := NewClient(origin.URL, testTokenID, "", origin.Client())
		require.NoError(t, clientErr)

		var result map[string]any

		response, err := DoRequest(
			t.Context(), client, http.MethodPost, "volumes", http.StatusAccepted,
			map[string]int{"size": 1}, &result,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindUnexpected), "unexpected error: %v", err)
		require.Equal(t, http.StatusTemporaryRedirect, response.StatusCode)
		require.True(t, reached)
		require.Empty(t, result, "the body of the redirect target must not be decoded")
	})
}

func TestClient_ContextEnds(t *testing.T) {
	t.Run("Canceled", func(t *testing.T) {
		httpClient := &testHTTPClient{err: &url.Error{Op: "Post", Err: context.Canceled}}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, nil,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindCanceled), "unexpected error: %v", err)
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("TimedOut", func(t *testing.T) {
		httpClient := &testHTTPClient{err: &url.Error{Op: "Get", Err: context.DeadlineExceeded}}

		_, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, nil,
		)

		require.Error(t, err)
		require.True(t, IsKind(err, KindTimeout), "unexpected error: %v", err)
	})

	t.Run("CanceledWhileReadingBody", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{
			status:  http.StatusOK,
			header:  http.Header{headerRequestID: []string{"req-partial-body"}},
			bodyErr: context.Canceled,
		}}}

		var result map[string]any

		meta, err := DoRequest(
			t.Context(), testClient(httpClient), http.MethodGet, "volumes", http.StatusOK, nil, &result,
		)

		require.Error(t, err)

		var sdkErr *Error
		require.ErrorAs(t, err, &sdkErr)

		require.Equal(t, KindCanceled, sdkErr.Kind, "unexpected error: %v", err)
		require.Equal(t, http.StatusOK, sdkErr.StatusCode)
		require.Equal(t, "req-partial-body", sdkErr.RequestID)
		require.Contains(t, sdkErr.Message, "unable to read the response body")
		require.Equal(t, http.StatusOK, meta.StatusCode)
	})
}
