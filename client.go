package blockstorage

import (
	"net"
	"net/http"
	"time"
)

const (
	// defaultHTTPTimeout represents the default timeout (in seconds) for HTTP requests.
	defaultHTTPTimeout = 120

	// defaultDialTimeout represents the default timeout (in seconds) for HTTP connection establishments.
	defaultDialTimeout = 60

	// defaultKeepaliveTimeout represents the default keep-alive period for an active network connection.
	defaultKeepaliveTimeout = 60

	// defaultMaxIdleConns represents the maximum number of idle (keep-alive) connections.
	defaultMaxIdleConns = 100

	// defaultIdleConnTimeout represents the maximum amount of time an idle (keep-alive) connection will remain
	// idle before closing itself.
	defaultIdleConnTimeout = 100

	// defaultTLSHandshakeTimeout represents the default timeout (in seconds) for TLS handshake.
	defaultTLSHandshakeTimeout = 60

	// defaultExpectContinueTimeout represents the default amount of time to wait for a server's first
	// response headers.
	defaultExpectContinueTimeout = 1
)

// ServiceClient stores details that are needed to work with the Selectel Block Storage API.
//
// The client is scoped to a single Cinder v3 endpoint of a single region and to a single
// project-scoped token. Both of them are provided by the caller.
type ServiceClient struct {
	// httpClient represents an initialized HTTP client that will be used to do requests.
	httpClient *http.Client

	// tokenID is a project-scoped authentication token.
	tokenID string

	// endpoint represents a regional Cinder v3 endpoint that will be used in all requests.
	endpoint string

	// userAgent contains a user agent that will be used in all requests.
	userAgent string
}

// NewClient initializes a new Selectel Block Storage API client.
//
// The endpoint is a regional Cinder v3 endpoint from the service catalog and the token
// is a project-scoped Keystone token. The SDK does not obtain either of them on its own.
func NewClient(endpoint, tokenID string, options ...ServiceClientOption) *ServiceClient {
	client := &ServiceClient{
		endpoint: endpoint,
		tokenID:  tokenID,
	}

	for _, option := range options {
		option(client)
	}

	if client.httpClient == nil {
		client.httpClient = newHTTPClient()
	}

	client.userAgent = buildUserAgent(client.userAgent)

	return client
}

// Endpoint returns the Cinder v3 endpoint that the client sends requests to.
func (client *ServiceClient) Endpoint() string {
	return client.endpoint
}

// UserAgent returns the User-Agent that the client sets on every request.
func (client *ServiceClient) UserAgent() string {
	return client.userAgent
}

// newHTTPClient returns a reference to an initialized and configured HTTP client.
func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   defaultHTTPTimeout * time.Second,
		Transport: newHTTPTransport(),
	}
}

// newHTTPTransport returns a reference to an initialized and configured HTTP transport.
func newHTTPTransport() *http.Transport {
	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   defaultDialTimeout * time.Second,
			KeepAlive: defaultKeepaliveTimeout * time.Second,
		}).DialContext,
		MaxIdleConns:          defaultMaxIdleConns,
		IdleConnTimeout:       defaultIdleConnTimeout * time.Second,
		TLSHandshakeTimeout:   defaultTLSHandshakeTimeout * time.Second,
		ExpectContinueTimeout: defaultExpectContinueTimeout * time.Second,
	}
}
