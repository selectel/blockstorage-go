package volumetype

import (
	"net/http"
	"testing"

	v1 "github.com/selectel/blockstorage-go/pkg/v1"
	"github.com/stretchr/testify/require"
)

func TestVolumeType_ListQoSLimits(t *testing.T) {
	t.Run("ReadsTypeEntries", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{status: http.StatusOK, body: `{
			"flexible.frontend-name": {
				"volume_type_id": "type-b",
				"qos_specs": {"total_iops_sec_min": 1000, "total_iops_sec_max": 50000},
				"allow_user_qos": true,
				"full_qos_disk_type": true
			},
			"basic.ru-1a": {"volume_type_id": "type-a", "qos_specs": {"total_iops_sec": 2000}},
			"region_volume_types": ["universal2", "fast2"],
			"cfg_timeout": 60
		}`}}}

		limits, response, err := ListQoSLimits(t.Context(), testClient(httpClient))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Len(t, limits, 2)

		request := httpClient.lastRequest(t)
		require.Equal(t, http.MethodGet, request.Method)
		require.Equal(t, "/volume/v3/"+testProjectID+"/qos-specs/qos_limits", request.URL.Path)

		require.Equal(t, "type-a", limits[0].VolumeTypeID)
		require.False(t, limits[0].AllowUserQoS)
		require.Equal(t, "type-b", limits[1].VolumeTypeID)
		require.Equal(t, 50000, limits[1].QoSSpecs["total_iops_sec_max"])
		require.True(t, limits[1].AllowUserQoS)
		require.True(t, limits[1].FullQoSDiskType)
	})

	t.Run("RejectsMalformedTypeEntry", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{
			status: http.StatusOK,
			body:   `{"broken.ru-1a": {"qos_specs": {"total_iops_sec": 2000}}, "cfg_timeout": 60}`,
		}}}

		limits, response, err := ListQoSLimits(t.Context(), testClient(httpClient))

		require.Nil(t, limits)
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.True(t, v1.IsKind(err, v1.KindUnexpected), "unexpected error: %v", err)
	})

	t.Run("PreservesAPIErrorClass", func(t *testing.T) {
		httpClient := &testHTTPClient{answers: []testAnswer{{
			status: http.StatusForbidden,
			body:   apiForbiddenJSON,
		}}}

		limits, response, err := ListQoSLimits(t.Context(), testClient(httpClient))

		require.Nil(t, limits)
		require.Equal(t, http.StatusForbidden, response.StatusCode)
		require.True(t, v1.IsKind(err, v1.KindForbidden), "unexpected error: %v", err)
	})
}
