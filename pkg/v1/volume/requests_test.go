package volume

import (
	"context"
	"net/http"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

func TestVolume_Create(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: apiVolumeJSON}}}

		created, response, err := Create(t.Context(), testClient(httpClient), CreateOpts{
			Size:             10,
			Name:             "data",
			Description:      "a volume",
			AvailabilityZone: "ru-1a",
			VolumeType:       "universal.ru-1a",
			Metadata:         map[string]string{testOwnerMetadataKey: testTeamMetadataValue},
		})
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, volumesPath(), request.URL.Path)
		require.Empty(t, request.Header.Get(headerAPIVersion))
		require.JSONEq(t, `{"volume": {
			"size": 10,
			"name": "data",
			"description": "a volume",
			"availability_zone": "ru-1a",
			"volume_type": "universal.ru-1a",
			"metadata": {"owner": "team"}
		}}`, httpClient.lastBody(t))

		require.Equal(t, http.StatusAccepted, response.StatusCode)
		require.Equal(t, testVolumeID, created.ID)
		require.Equal(t, "creating", created.Status)

		require.Equal(t, "universal.ru-1a", created.VolumeType)
		require.Equal(
			t,
			map[string]string{testOwnerMetadataKey: testTeamMetadataValue, "total_iops_sec": "3000"},
			created.Metadata,
		)
	})

	t.Run("Sources", func(t *testing.T) {
		cases := []struct {
			name     string
			opts     CreateOpts
			expected string
		}{
			{"FromSnapshot", CreateOpts{SnapshotID: "snap-1"}, `{"volume": {"snapshot_id": "snap-1"}}`},
			{"FromVolume", CreateOpts{SourceVolID: testSourceVolumeID}, `{"volume": {"source_volid": "vol-1"}}`},
			{"FromImage", CreateOpts{ImageID: "image-1"}, `{"volume": {"imageRef": "image-1"}}`},
			{"FromBackup", CreateOpts{BackupID: testBackupID}, `{"volume": {"backup_id": "backup-1"}}`},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: apiVolumeJSON}}}

				_, _, err := Create(t.Context(), testClient(httpClient), testCase.opts)
				require.NoError(t, err)
				require.JSONEq(t, testCase.expected, httpClient.lastBody(t))
			})
		}
	})

	t.Run("ValidationDelegatedToCinder", func(t *testing.T) {
		cases := []struct {
			name     string
			opts     CreateOpts
			expected string
		}{
			{
				"TwoSources",
				CreateOpts{Size: 1, ImageID: "image-1", SnapshotID: "snap-1"},
				`{"volume":{"size":1,"imageRef":"image-1","snapshot_id":"snap-1"}}`,
			},
			{"NoSizeAndNoSource", CreateOpts{Name: "data"}, `{"volume":{"name":"data"}}`},
			{"NegativeSize", CreateOpts{Size: -1}, `{"volume":{"size":-1}}`},
			{
				"ArbitrarySourceID",
				CreateOpts{SnapshotID: "not-a-uuid"},
				`{"volume":{"snapshot_id":"not-a-uuid"}}`,
			},
		}

		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				httpClient := &testHTTPClient{
					answers: []testAnswer{{status: http.StatusAccepted, body: apiVolumeJSON}},
				}

				_, _, err := Create(t.Context(), testClient(httpClient), testCase.opts)

				require.NoError(t, err)
				require.Len(t, httpClient.requests, 1)
				require.JSONEq(t, testCase.expected, httpClient.lastBody(t))
			})
		}
	})

	t.Run("MicroversionOfCreateFromBackup", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: apiVolumeJSON}}}

		_, _, err := Create(t.Context(), testClient(httpClient), CreateOpts{BackupID: testBackupID})
		require.NoError(t, err)

		require.Equal(t, "volume "+microversionCreateFromBackup,
			httpClient.lastRequest(t).Header.Get(headerAPIVersion))
	})

	t.Run("MicroversionRejectedByEndpoint", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusNotAcceptable, body: apiMicroversionRefusedJSON},
		}}

		_, _, err := Create(t.Context(), testClient(httpClient), CreateOpts{BackupID: testBackupID})

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindMicroversion), "unexpected error: %v", err)
	})
}

func TestVolume_Get(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiAttachedVolumeJSON}}}

		found, response, err := Get(t.Context(), testClient(httpClient), testVolumeID)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, volumesPath()+"/"+testVolumeID, request.URL.Path)
		require.Empty(t, httpClient.lastBody(t))

		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "in-use", found.Status)
		require.Len(t, found.Attachments, 1)

		require.Equal(t, testVolumeID, found.Attachments[0].ID)
		require.Equal(t, testAttachmentID, found.Attachments[0].AttachmentID)
		require.Equal(t, testServerID, found.Attachments[0].ServerID)
		require.Equal(t, "/dev/vdb", found.Attachments[0].Device)
	})

	t.Run("Missing", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusNotFound, body: apiNotFoundJSON}}}

		_, response, err := Get(t.Context(), testClient(httpClient), testVolumeID)

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindNotFound))
		require.Equal(t, http.StatusNotFound, response.StatusCode)
	})
}

func TestVolume_GetRejectsInvalidEnvelope(t *testing.T) {
	httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: `{"other": {}}`}}}

	found, response, err := Get(t.Context(), testClient(httpClient), testVolumeID)

	require.Nil(t, found)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
}

func TestVolume_RejectsUnexpectedSuccessStatus(t *testing.T) {
	name := "status-contract-name"
	testCases := []struct {
		name   string
		status int
		call   func(context.Context, *v1.Client) error
	}{
		{
			name:   "CreateRequires202",
			status: http.StatusOK,
			call: func(ctx context.Context, client *v1.Client) error {
				_, _, err := Create(ctx, client, CreateOpts{Name: "disk"})
				return err
			},
		},
		{
			name:   "GetRequires200",
			status: http.StatusNoContent,
			call: func(ctx context.Context, client *v1.Client) error {
				_, _, err := Get(ctx, client, testVolumeID)
				return err
			},
		},
		{
			name:   "UpdateRequires200",
			status: http.StatusAccepted,
			call: func(ctx context.Context, client *v1.Client) error {
				_, _, err := Update(ctx, client, testVolumeID, UpdateOpts{Name: &name})
				return err
			},
		},
		{
			name:   "ExtendRequires202",
			status: http.StatusNoContent,
			call: func(ctx context.Context, client *v1.Client) error {
				_, err := Extend(ctx, client, testVolumeID, ExtendOpts{NewSize: 20})
				return err
			},
		},
		{
			name:   "DeleteRequires202",
			status: http.StatusNoContent,
			call: func(ctx context.Context, client *v1.Client) error {
				_, err := Delete(ctx, client, testVolumeID)
				return err
			},
		},
		{
			name:   "ListRequires200",
			status: http.StatusPartialContent,
			call: func(ctx context.Context, client *v1.Client) error {
				_, err := ListDetail(ctx, client, ListOpts{})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			httpClient := &testHTTPClient{answers: []testAnswer{{status: testCase.status, body: apiVolumeJSON}}}

			err := testCase.call(t.Context(), testClient(httpClient))

			require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
		})
	}
}

func TestVolume_Update(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiVolumeJSON}}}

		name := "renamed"
		description := ""

		_, response, err := Update(t.Context(), testClient(httpClient), testVolumeID, UpdateOpts{
			Name:        &name,
			Description: &description,
			Metadata:    map[string]string{testOwnerMetadataKey: "other"},
		})
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodPut, request.Method)
		require.Equal(t, volumesPath()+"/"+testVolumeID, request.URL.Path)

		require.JSONEq(t, `{"volume": {"name": "renamed", "description": "", "metadata": {"owner": "other"}}}`,
			httpClient.lastBody(t))
		require.Equal(t, http.StatusOK, response.StatusCode)
	})

	t.Run("OmitsUnsetFields", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiVolumeJSON}}}

		name := "renamed"

		_, _, err := Update(t.Context(), testClient(httpClient), testVolumeID, UpdateOpts{Name: &name})
		require.NoError(t, err)

		require.JSONEq(t, `{"volume": {"name": "renamed"}}`, httpClient.lastBody(t))
	})

	t.Run("ClearsMetadata", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: apiVolumeJSON}}}

		_, _, err := Update(
			t.Context(),
			testClient(httpClient),
			testVolumeID,
			UpdateOpts{Metadata: map[string]string{}},
		)
		require.NoError(t, err)

		require.JSONEq(t, `{"volume": {"metadata": {}}}`, httpClient.lastBody(t))
	})

	t.Run("WithoutChanges", func(t *testing.T) {
		httpClient := &testHTTPClient{}

		_, _, err := Update(t.Context(), testClient(httpClient), testVolumeID, UpdateOpts{})

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindInvalidRequest))
		require.Empty(t, httpClient.requests)
	})
}

func TestVolume_Extend(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: ""}}}

		response, err := Extend(t.Context(), testClient(httpClient), testVolumeID, ExtendOpts{NewSize: 20})
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodPost, request.Method)
		require.Equal(t, volumesPath()+"/"+testVolumeID+"/action", request.URL.Path)
		require.JSONEq(t, `{"os-extend": {"new_size": 20}}`, httpClient.lastBody(t))

		require.Empty(t, request.Header.Get(headerAPIVersion))
		require.Equal(t, http.StatusAccepted, response.StatusCode)
	})

	t.Run("MicroversionOfAttachedExtend", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: ""}}}

		_, err := Extend(t.Context(), testClient(httpClient), testVolumeID, ExtendOpts{NewSize: 20, Attached: true})
		require.NoError(t, err)

		require.Equal(t, "volume "+microversionOnlineResize,
			httpClient.lastRequest(t).Header.Get(headerAPIVersion))
	})

	t.Run("RejectedLocally", func(t *testing.T) {
		httpClient := &testHTTPClient{}

		_, err := Extend(t.Context(), testClient(httpClient), testVolumeID, ExtendOpts{NewSize: 0})

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindInvalidRequest))
		require.Empty(t, httpClient.requests)
	})
}

func TestVolume_Delete(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusAccepted, body: ""}}}

		response, err := Delete(t.Context(), testClient(httpClient), testVolumeID)
		require.NoError(t, err)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodDelete, request.Method)
		require.Equal(t, volumesPath()+"/"+testVolumeID, request.URL.Path)
		require.Empty(t, httpClient.lastBody(t))
		require.Equal(t, http.StatusAccepted, response.StatusCode)
	})

	t.Run("Attached", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusBadRequest, body: apiAttachedDeleteRefusedJSON},
		}}

		_, err := Delete(t.Context(), testClient(httpClient), testVolumeID)

		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindInvalidRequest), "unexpected error: %v", err)
	})
}

func TestVolume_RejectsEmptyID(t *testing.T) {
	httpClient := &testHTTPClient{}
	client := testClient(httpClient)

	_, _, getErr := Get(t.Context(), client, "")
	_, _, updateErr := Update(t.Context(), client, "", UpdateOpts{Metadata: map[string]string{}})
	_, extendErr := Extend(t.Context(), client, "", ExtendOpts{NewSize: 1})
	_, deleteErr := Delete(t.Context(), client, "")

	for _, err := range []error{getErr, updateErr, extendErr, deleteErr} {
		require.Error(t, err)
		require.True(t, v1.IsKind(err, v1.KindInvalidRequest), "unexpected error: %v", err)
	}

	require.Empty(t, httpClient.requests)
}

func TestVolume_ListDetail(t *testing.T) {
	t.Run("ReadsEveryPage", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{
			{status: http.StatusOK, body: apiVolumePage([]string{testSourceVolumeID, "vol-2"}, "vol-2")},
			{status: http.StatusOK, body: apiVolumePage([]string{"vol-3", "vol-4"}, "vol-4")},
			{status: http.StatusOK, body: apiVolumePage(nil, "")},
		}}

		volumes, err := ListDetail(t.Context(), testClient(httpClient), ListOpts{Limit: 2})
		require.NoError(t, err)

		require.Len(t, volumes, 4)
		require.Equal(t, testSourceVolumeID, volumes[0].ID)
		require.Equal(t, "vol-4", volumes[3].ID)
		require.Equal(t, map[string]string{testOwnerMetadataKey: testTeamMetadataValue}, volumes[0].Metadata)

		require.Len(t, httpClient.requests, 3)
		require.Equal(t, volumesPath()+"/detail", httpClient.requests[0].URL.Path)
		require.Equal(t, "limit=2", httpClient.requests[0].URL.RawQuery)

		require.Equal(t, "limit=2&marker=vol-2", httpClient.requests[1].URL.RawQuery)
		require.Equal(t, "limit=2&marker=vol-4", httpClient.requests[2].URL.RawQuery)
	})
}
