package pagination

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint = "https://api.example.com/volume/v3/test-project-id"
	testTokenID  = "fake-project-scoped-token" //nolint:gosec // G101: this is a fake value used by the tests.
)

type testItem struct {
	ID string `json:"id"`
}

type testPage struct {
	Values []testItem `json:"items"`
	Links  []Link     `json:"links"`
}

func (p *testPage) Items() []testItem { return p.Values }

func (p *testPage) NextHref() string { return NextHref(p.Links) }

type testAnswer struct {
	status int
	body   string
}

type testHTTPClient struct {
	answers  []testAnswer
	requests []*http.Request
}

func (client *testHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.requests = append(client.requests, request)

	answer := client.answers[0]
	client.answers = client.answers[1:]

	return &http.Response{
		StatusCode: answer.status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(answer.body)),
	}, nil
}

func TestReadAll(t *testing.T) {
	t.Run("CombinesQueryAndLimit", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: `{"items":[]}`}}}

		_, err := ReadAll[testItem](
			t.Context(), testClient(t, httpClient), "items", "items",
			url.Values{"status": []string{"available"}}, 5, newTestPage,
		)
		require.NoError(t, err)
		require.Len(t, httpClient.requests, 1)
		require.Equal(t, "limit=5&status=available", httpClient.requests[0].URL.RawQuery)
	})

	t.Run("ReadsEveryPageOnTheConfiguredEndpoint", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{
				status: http.StatusOK,
				body: `{"items":[{"id":"item-1"}],"links":[{"rel":"next",` +
					`"href":"https://another.example.com/volume/v3/test-project-id/items?limit=1&marker=item-1"}]}`,
			},
			{status: http.StatusOK, body: pageBody("item-2", testEndpoint+"/items?limit=1&marker=item-2")},
			{status: http.StatusOK, body: `{"items":[]}`},
		}}

		items, err := ReadAll[testItem](t.Context(), testClient(t, httpClient), "items", "items", nil, 1, newTestPage)
		require.NoError(t, err)
		require.Equal(t, []testItem{{ID: "item-1"}, {ID: "item-2"}}, items)
		require.Len(t, httpClient.requests, 3)

		require.Equal(t, "/volume/v3/test-project-id/items", httpClient.requests[0].URL.Path)
		require.Equal(t, "limit=1", httpClient.requests[0].URL.RawQuery)

		require.Equal(t, "api.example.com", httpClient.requests[1].URL.Host)
		require.Equal(t, "/volume/v3/test-project-id/items", httpClient.requests[1].URL.Path)
		require.Equal(t, "limit=1&marker=item-1", httpClient.requests[1].URL.RawQuery)
		require.Equal(t, "limit=1&marker=item-2", httpClient.requests[2].URL.RawQuery)
	})

	t.Run("ReportsAnIncompleteListWithTheCause", func(t *testing.T) {
		next := testEndpoint + "/items?limit=1&marker=item-1"
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: pageBody("item-1", next)},
			{
				status: http.StatusForbidden,
				body:   `{"forbidden":{"code":403,"message":"Policy does not allow this."}}`,
			},
		}}

		items, err := ReadAll[testItem](t.Context(), testClient(t, httpClient), "items", "items", nil, 1, newTestPage)

		require.Nil(t, items)
		require.True(t, v1.IsKind(err, v1.KindIncompleteList), "unexpected error: %v", err)
		require.True(t, v1.IsKind(err, v1.KindForbidden), "unexpected error: %v", err)
		require.Len(t, httpClient.requests, 2)
	})

	t.Run("RefusesARepeatedPage", func(t *testing.T) {
		next := testEndpoint + "/items?limit=1&marker=item-1"
		page := pageBody("item-1", next)
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: page},
			{status: http.StatusOK, body: page},
		}}

		items, err := ReadAll[testItem](t.Context(), testClient(t, httpClient), "items", "items", nil, 1, newTestPage)

		require.Nil(t, items)
		require.True(t, v1.IsKind(err, v1.KindIncompleteList), "unexpected error: %v", err)
		require.Len(t, httpClient.requests, 2)
	})
}

func testClient(t *testing.T, httpClient *testHTTPClient) *v1.Client {
	t.Helper()

	client, err := v1.NewClient(v1.Config{
		Endpoint:   testEndpoint,
		Token:      testTokenID,
		HTTPClient: httpClient,
	})
	require.NoError(t, err)

	return client
}

func newTestPage() *testPage { return &testPage{} }

func pageBody(id, next string) string {
	return `{"items":[{"id":"` + id + `"}],"links":[{"rel":"next","href":"` + next + `"}]}`
}
