package snapshot_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/selectel/blockstorage-go/pkg/v1/snapshot"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint   = "https://api.example.com/volume/v3/project-id"
	testToken      = "fake-project-scoped-token" //nolint:gosec // G101: fake test value.
	testSnapshotID = "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8"
	testVolumeID   = "99887766-5544-3322-1100-aabbccddeeff"
)

type httpClientFunc func(*http.Request) (*http.Response, error)

func (do httpClientFunc) Do(request *http.Request) (*http.Response, error) { return do(request) }

type testAnswer struct {
	status int
	body   string
	err    error
}

type testHTTPClient struct {
	answers  []testAnswer
	requests []*http.Request
}

func (client *testHTTPClient) Do(request *http.Request) (*http.Response, error) {
	client.requests = append(client.requests, request)

	answer := client.answers[0]
	client.answers = client.answers[1:]
	if answer.err != nil {
		return nil, answer.err
	}

	return response(answer.status, answer.body), nil
}

func TestGet(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/volume/v3/project-id/snapshots/"+testSnapshotID, request.URL.Path)

		return response(http.StatusOK, `{"snapshot": {
			"id": "`+testSnapshotID+`",
			"created_at": "2025-03-04T05:06:07.123456",
			"name": "daily",
			"description": "daily backup",
			"volume_id": "`+testVolumeID+`",
			"status": "available",
			"size": 10,
			"metadata": {"owner": "team"}
		}}`), nil
	})

	view, meta, err := snapshot.Get(t.Context(), client, testSnapshotID)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, meta.StatusCode)
	require.Equal(t, testSnapshotID, view.ID)
	require.Equal(t, time.Date(2025, 3, 4, 5, 6, 7, 123456000, time.UTC), view.CreatedAt)
	require.Equal(t, "daily", view.Name)
	require.Equal(t, "daily backup", view.Description)
	require.Equal(t, testVolumeID, view.VolumeID)
	require.Equal(t, "available", view.Status)
	require.Equal(t, 10, view.Size)
	require.Equal(t, map[string]string{"owner": "team"}, view.Metadata)
}

func TestGetRejectsEmptyID(t *testing.T) {
	view, meta, err := snapshot.Get(t.Context(), nil, "")

	require.Nil(t, view)
	require.Nil(t, meta)
	require.True(t, v1.IsKind(err, v1.KindInvalidRequest))
}

func TestListFilters(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/volume/v3/project-id/snapshots", request.URL.Path)
		require.Equal(t, "daily", request.URL.Query().Get("name"))
		require.Equal(t, "available", request.URL.Query().Get("status"))
		require.Equal(t, testVolumeID, request.URL.Query().Get("volume_id"))

		return response(http.StatusOK, `{"snapshots": [{
			"id": "`+testSnapshotID+`",
			"created_at": "2025-03-04T05:06:07.123456",
			"name": "daily"
		}]}`), nil
	})

	views, err := snapshot.List(t.Context(), client, snapshot.ListOpts{
		Name: "daily", Status: "available", VolumeID: testVolumeID,
	})

	require.NoError(t, err)
	require.Len(t, views, 1)
	require.Equal(t, testSnapshotID, views[0].ID)
}

func TestListOmitsEmptyFilters(t *testing.T) {
	client := testClient(t, func(request *http.Request) (*http.Response, error) {
		require.Empty(t, request.URL.RawQuery)

		return response(http.StatusOK, `{"snapshots": []}`), nil
	})

	views, err := snapshot.List(t.Context(), client, snapshot.ListOpts{})

	require.NoError(t, err)
	require.Empty(t, views)
}

func TestListFollowsSnapshotLinks(t *testing.T) {
	next := testEndpoint + "/snapshots?marker=snapshot-1"
	httpClient := &testHTTPClient{answers: []testAnswer{
		{status: http.StatusOK, body: snapshotPage("snapshot-1", next)},
		{status: http.StatusOK, body: `{"snapshots":[{"id":"snapshot-2"}]}`},
	}}

	views, err := snapshot.List(t.Context(), testClient(t, httpClient.Do), snapshot.ListOpts{})

	require.NoError(t, err)
	require.Len(t, views, 2)
	require.Equal(t, []string{"snapshot-1", "snapshot-2"}, []string{views[0].ID, views[1].ID})
	require.Len(t, httpClient.requests, 2)
	require.Equal(t, "snapshot-1", httpClient.requests[1].URL.Query().Get("marker"))
}

func TestListRejectsIncompleteResults(t *testing.T) {
	next := testEndpoint + "/snapshots?marker=snapshot-1"
	tests := []struct {
		name   string
		answer testAnswer
		kind   v1.Kind
	}{
		{
			name:   "transport error",
			answer: testAnswer{err: errors.New("connection reset")},
			kind:   v1.KindTransport,
		},
		{
			name: "HTTP error",
			answer: testAnswer{
				status: http.StatusForbidden,
				body:   `{"forbidden":{"code":403,"message":"Policy does not allow this."}}`,
			},
			kind: v1.KindForbidden,
		},
		{
			name:   "decode error",
			answer: testAnswer{status: http.StatusOK, body: `{`},
			kind:   v1.KindUnexpected,
		},
		{
			name:   "repeated page",
			answer: testAnswer{status: http.StatusOK, body: snapshotPage("snapshot-1", next)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			httpClient := &testHTTPClient{answers: []testAnswer{
				{status: http.StatusOK, body: snapshotPage("snapshot-1", next)},
				test.answer,
			}}

			views, err := snapshot.List(t.Context(), testClient(t, httpClient.Do), snapshot.ListOpts{})

			require.Nil(t, views)
			require.True(t, v1.IsKind(err, v1.KindIncompleteList), "unexpected error: %v", err)
			if test.kind != "" {
				require.True(t, v1.IsKind(err, test.kind), "unexpected error: %v", err)
			}
			require.Len(t, httpClient.requests, 2)
		})
	}
}

func testClient(t *testing.T, do func(*http.Request) (*http.Response, error)) *v1.Client {
	t.Helper()

	client, err := v1.NewClient(v1.Config{
		Endpoint:   testEndpoint,
		Token:      testToken,
		HTTPClient: httpClientFunc(do),
	})
	require.NoError(t, err)

	return client
}

func snapshotPage(id, next string) string {
	return `{"snapshots":[{"id":"` + id + `"}],"snapshots_links":[{"rel":"next","href":"` + next + `"}]}`
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
