package volumetype

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

const (
	testTokenID   = "fake-project-scoped-token" //nolint:gosec // G101: this is a fake value used by the tests.
	testProjectID = "test-project-id"
	testEndpoint  = "https://api.example.com/volume/v3/" + testProjectID

	testTypeID      = "7b3c1d2e-4f50-6172-8394-a5b6c7d8e9f0"
	testFirstTypeID = "type-1"
)

const (
	apiTypeJSON = `{"volume_type": {
		"id": "7b3c1d2e-4f50-6172-8394-a5b6c7d8e9f0",
		"name": "universal.ru-1a",
		"description": "Universal SSD",
		"is_public": true,
		"extra_specs": {"RESKEY:availability_zones": "ru-1a", "multiattach": "<is> False"}
	}}`

	apiDefaultTypeJSON = `{"volume_type": {
		"id": "0a1b2c3d-4e5f-6071-8293-a4b5c6d7e8f9",
		"name": "__DEFAULT__.__DEFAULT-ZONE__",
		"description": null,
		"is_public": true,
		"extra_specs": {}
	}}`

	apiAdminTypeJSON = `{"volume_type": {
		"id": "7b3c1d2e-4f50-6172-8394-a5b6c7d8e9f0",
		"name": "universal.ru-1a",
		"is_public": true,
		"extra_specs": {"volume_backend_name": "rbd-1", "RESKEY:availability_zones": "ru-1a"},
		"qos_specs_id": "9f8e7d6c-5b4a-3210-fedc-ba9876543210"
	}}`

	apiNotFoundJSON = `{"itemNotFound": {"code": 404, "message": "Volume type could not be found."}}`

	apiDefaultMisconfiguredJSON = `{"itemNotFound": {"code": 404, "message": ` +
		`"Volume type with name __DEFAULT__ could not be found."}}`

	apiForbiddenJSON = `{"forbidden": {"code": 403, "message": "Policy does not allow this to be performed."}}`

	apiFaultJSON = `{"computeFault": {"code": 500, "message": "The server has either erred or is ` +
		`incapable of performing the requested operation."}}`
)

type testAnswer struct {
	status  int
	body    string
	header  http.Header
	bodyErr error
}

type testHTTPClient struct {
	requests []*http.Request
	bodies   []string
	answers  []testAnswer
	err      error
}

func (client *testHTTPClient) Do(request *http.Request) (*http.Response, error) {
	body := ""
	if request.Body != nil {
		raw, _ := io.ReadAll(request.Body)
		body = string(raw)
	}

	client.requests = append(client.requests, request)
	client.bodies = append(client.bodies, body)

	if client.err != nil {
		return nil, client.err
	}

	answer := client.answers[0]
	if len(client.answers) > 1 {
		client.answers = client.answers[1:]
	}

	header := answer.header
	if header == nil {
		header = http.Header{}
	}

	var reader io.Reader = strings.NewReader(answer.body)
	if answer.bodyErr != nil {
		reader = errorReader{err: answer.bodyErr}
	}

	return &http.Response{StatusCode: answer.status, Header: header, Body: io.NopCloser(reader)}, nil
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func (client *testHTTPClient) lastRequest(t *testing.T) *http.Request {
	t.Helper()

	require.NotEmpty(t, client.requests)

	return client.requests[len(client.requests)-1]
}

func testClient(httpClient v1.HTTPClient) *v1.Client {
	client, err := v1.NewClient(v1.Config{
		Endpoint:   testEndpoint,
		Token:      testTokenID,
		HTTPClient: httpClient,
	})
	if err != nil {
		panic(err)
	}

	return client
}

func typesPath() string {
	return "/volume/v3/" + testProjectID + "/types"
}

func apiTypePage(ids []string, marker string) string {
	types := make([]string, 0, len(ids))
	for _, id := range ids {
		types = append(types, fmt.Sprintf(
			`{"id": %q, "name": %q, "is_public": true, "extra_specs": {"RESKEY:availability_zones": "ru-1a"}}`,
			id, id))
	}

	page := fmt.Sprintf(`{"volume_types": [%s]`, strings.Join(types, ","))
	if marker != "" {
		page += fmt.Sprintf(`, "volume_type_links": [{"rel": "next", "href": %q}]`,
			testEndpoint+"/types?limit=2&marker="+marker)
	}

	return page + "}"
}
