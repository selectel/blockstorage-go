package blockstorage

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testEndpoint = "https://api.example.com/volume/v3/test-project-id"
	testTokenID  = "fake-project-scoped-token"
)

func TestClientUsesProvidedScope(t *testing.T) {
	client := NewClient(testEndpoint, testTokenID)

	assert.Equal(t, testEndpoint, client.Endpoint())
	assert.Equal(t, testTokenID, client.tokenID)
	assert.NotNil(t, client.httpClient)
}

func TestClientDefaultUserAgent(t *testing.T) {
	client := NewClient(testEndpoint, testTokenID)

	assert.Equal(t, appName+"/"+unknownModuleVersion, client.UserAgent())
}

func TestClientUserAgentPrefix(t *testing.T) {
	client := NewClient(testEndpoint, testTokenID, WithUserAgentPrefix("terraform-provider-selectel/v7.0.0"))

	assert.Equal(t, "terraform-provider-selectel/v7.0.0 "+appName+"/"+unknownModuleVersion, client.UserAgent())
}

func TestClientCustomHTTPClient(t *testing.T) {
	httpClient, seen := newFakeHTTPClient(func(_ *http.Request) (*http.Response, error) {
		return newFakeResponse(http.StatusOK, "{}"), nil
	})

	client := NewClient(testEndpoint, testTokenID, WithCustomHTTPClient(httpClient))
	require.Same(t, httpClient, client.httpClient)

	assert.Empty(t, *seen)
}
