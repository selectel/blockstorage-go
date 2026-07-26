package volumetype

import (
	"errors"
	"net/http"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

func TestVolumeType_Get(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiTypeJSON}}}

		found, response, err := Get(t.Context(), testClient(httpClient), testTypeID)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, typesPath()+"/"+testTypeID, request.URL.Path)

		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, testTypeID, found.ID)
		require.Equal(t, "universal.ru-1a", found.Name)
		require.Equal(t, "Universal SSD", found.Description)
		require.True(t, found.IsPublic)
		require.Equal(t, map[string]string{
			"RESKEY:availability_zones": "ru-1a",
			"multiattach":               "<is> False",
		}, found.ExtraSpecs)
	})

	t.Run("Default", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiDefaultTypeJSON}}}

		found, _, err := Get(t.Context(), testClient(httpClient), DefaultTypeID)
		require.NoError(t, err)

		require.Equal(t, typesPath()+"/default", httpClient.lastRequest(t).URL.Path)

		require.Equal(t, "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9", found.ID)
		require.NotEqual(t, DefaultTypeID, found.ID)
		require.Empty(t, found.ExtraSpecs)
	})

	t.Run("ReportsVisibleExtraSpecs", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiAdminTypeJSON}}}

		found, _, err := Get(t.Context(), testClient(httpClient), testTypeID)
		require.NoError(t, err)

		require.Equal(t, "rbd-1", found.ExtraSpecs["volume_backend_name"])
	})

	t.Run("RejectsEmptyID", func(t *testing.T) {
		httpClient := &testHTTPClient{}

		_, _, err := Get(t.Context(), testClient(httpClient), "")

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindInvalidRequest))
		require.Empty(t, httpClient.requests, "the request must not reach the API")
	})

	t.Run("ErrorClasses", func(t *testing.T) {
		cases := []struct {
			name       string
			statusCode int
			body       string
			expected   v1.Kind
		}{
			{"MissingType", http.StatusNotFound, apiNotFoundJSON, v1.KindNotFound},
			{"MisconfiguredDefault", http.StatusNotFound, apiDefaultMisconfiguredJSON, v1.KindNotFound},
			{"ForbiddenRole", http.StatusForbidden, apiForbiddenJSON, v1.KindForbidden},
			{"ServerError", http.StatusInternalServerError, apiFaultJSON, v1.KindServerError},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				httpClient := &testHTTPClient{answers: []testAnswer{
					{status: testCase.statusCode, body: testCase.body},
				}}

				_, response, err := Get(t.Context(), testClient(httpClient), testTypeID)

				require.Error(t, err)
				require.True(t, v1.IsKind(err, testCase.expected), "unexpected error: %v", err)
				require.Equal(t, testCase.statusCode, response.StatusCode)
			})
		}
	})

	t.Run("TransportFailure", func(t *testing.T) {
		httpClient := &testHTTPClient{err: errors.New("connection reset by peer")}

		_, _, err := Get(t.Context(), testClient(httpClient), testTypeID)

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindTransport), "unexpected error: %v", err)
		require.Len(t, httpClient.requests, 1, "the SDK must not retry the request")
	})
}

func TestVolumeType_GetRejectsInvalidEnvelope(t *testing.T) {
	httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: `{"other": {}}`}}}

	found, response, err := Get(t.Context(), testClient(httpClient), testTypeID)

	require.Nil(t, found)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
}

func TestVolumeType_RejectsUnexpectedSuccessStatus(t *testing.T) {
	t.Run("GetRequires200", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusNoContent}}}

		_, response, err := Get(t.Context(), testClient(httpClient), testTypeID)

		require.Equal(t, http.StatusNoContent, response.StatusCode)
		require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
	})

	t.Run("ListRequires200", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{
			status: http.StatusPartialContent,
			body:   `{"volume_types": []}`,
		}}}

		_, err := List(t.Context(), testClient(httpClient), ListOpts{})

		require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
	})
}

func TestVolumeType_List(t *testing.T) {
	t.Run("ReadsEveryPage", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: apiTypePage([]string{testFirstTypeID, "type-2"}, "type-2")},
			{status: http.StatusOK, body: apiTypePage([]string{"type-3"}, "type-3")},
			{status: http.StatusOK, body: apiTypePage(nil, "")},
		}}

		types, err := List(t.Context(), testClient(httpClient), ListOpts{Limit: 2})
		require.NoError(t, err)

		require.Len(t, types, 3)
		require.Equal(t, testFirstTypeID, types[0].ID)
		require.Equal(t, "type-3", types[2].ID)
		require.Equal(t, map[string]string{"RESKEY:availability_zones": "ru-1a"}, types[0].ExtraSpecs)

		require.Len(t, httpClient.requests, 3)
		require.Equal(t, typesPath(), httpClient.requests[0].URL.Path)
		require.Equal(t, "limit=2", httpClient.requests[0].URL.RawQuery)
		require.Equal(t, "limit=2&marker=type-2", httpClient.requests[1].URL.RawQuery)
		require.Equal(t, "limit=2&marker=type-3", httpClient.requests[2].URL.RawQuery)
	})

	t.Run("WithoutLinks", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: `{"volume_types": [{"id": "type-1", "name": "basic.ru-1a"}]}`},
		}}

		types, err := List(t.Context(), testClient(httpClient), ListOpts{})
		require.NoError(t, err)

		require.Len(t, types, 1)
		require.Len(t, httpClient.requests, 1)
		require.Empty(t, httpClient.requests[0].URL.RawQuery)
	})
}
