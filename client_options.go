package blockstorage

import "net/http"

// ServiceClientOption is a functional parameter of NewClient.
type ServiceClientOption func(*ServiceClient)

// WithCustomHTTPClient is a functional parameter for the client, used to set a custom HTTP client
// with a caller-owned transport, timeouts and instrumentation.
func WithCustomHTTPClient(httpClient *http.Client) ServiceClientOption {
	return func(client *ServiceClient) {
		client.httpClient = httpClient
	}
}

// WithUserAgentPrefix is a functional parameter for the client, used to set a custom User-Agent
// prefix that will be placed before the SDK name and version.
//
// It is highly recommended to use this option.
func WithUserAgentPrefix(prefix string) ServiceClientOption {
	return func(client *ServiceClient) {
		client.userAgent = prefix
	}
}
