package volume

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

	headerAPIVersion = "OpenStack-API-Version"

	testVolumeID          = "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8"
	testAttachmentID      = "aa11bb22-cc33-dd44-ee55-ff6677889900"
	testServerID          = "99887766-5544-3322-1100-aabbccddeeff"
	testSourceVolumeID    = "vol-1"
	testBackupID          = "backup-1"
	testOwnerMetadataKey  = "owner"
	testTeamMetadataValue = "team"
)

const (
	apiVolumeJSON = `{"volume": {
		"id": "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8",
		"status": "creating",
		"size": 10,
		"name": "data",
		"description": "a volume",
		"availability_zone": "ru-1a",
		"volume_type": "universal.ru-1a",
		"metadata": {"owner": "team", "total_iops_sec": "3000"},
		"attachments": [],
		"bootable": "false",
		"multiattach": false
	}}`

	apiAttachedVolumeJSON = `{"volume": {
		"id": "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8",
		"status": "in-use",
		"size": 10,
		"attachments": [{
			"id": "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8",
			"attachment_id": "aa11bb22-cc33-dd44-ee55-ff6677889900",
			"volume_id": "0e1c2b3a-4d5e-6f70-8192-a3b4c5d6e7f8",
			"server_id": "99887766-5544-3322-1100-aabbccddeeff",
			"device": "/dev/vdb",
			"host_name": null
		}]
	}}`

	apiNotFoundJSON = `{"itemNotFound": {"code": 404, "message": "Volume could not be found."}}`

	apiAttachedDeleteRefusedJSON = `{"badRequest": {"code": 400, "message": "Invalid volume: Volume status ` +
		`must be available, error, error_restoring, error_extending, error_managing and must not be ` +
		`migrating, attached, belong to a group, have snapshots, awaiting a transfer."}}`

	apiMicroversionRefusedJSON = `{"computeFault": {"code": 406, "message": "Version 3.47 is not supported ` +
		`by the API. Minimum is 3.0 and maximum is 3.70."}}`
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

func (client *testHTTPClient) lastBody(t *testing.T) string {
	t.Helper()

	require.NotEmpty(t, client.bodies)

	return client.bodies[len(client.bodies)-1]
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

func volumesPath() string {
	return "/volume/v3/" + testProjectID + "/volumes"
}

func apiVolumePage(ids []string, marker string) string {
	volumes := make([]string, 0, len(ids))
	for _, id := range ids {
		volumes = append(volumes, fmt.Sprintf(`{"id": %q, "metadata": {"owner": "team"}}`, id))
	}

	page := fmt.Sprintf(`{"volumes": [%s]`, strings.Join(volumes, ","))
	if marker != "" {
		page += fmt.Sprintf(`, "volumes_links": [{"rel": "next", "href": %q}]`,
			testEndpoint+"/volumes/detail?limit=2&marker="+marker)
	}

	return page + "}"
}
