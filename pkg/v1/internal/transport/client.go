package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultHTTPTimeout           = 120
	defaultDialTimeout           = 60
	defaultKeepaliveTimeout      = 60
	defaultMaxIdleConns          = 100
	defaultIdleConnTimeout       = 100
	defaultTLSHandshakeTimeout   = 60
	defaultExpectContinueTimeout = 1
)

const (
	headerAuthToken   = "X-Auth-Token" //nolint:gosec // G101: this is an HTTP header name, not a credential.
	headerUserAgent   = "User-Agent"
	headerAccept      = "Accept"
	headerContentType = "Content-Type"
	headerAPIVersion  = "OpenStack-API-Version"
	apiVersionService = "volume"
	contentTypeJSON   = "application/json"
)

type HTTPClient interface {
	Do(request *http.Request) (*http.Response, error)
}

type Client struct {
	httpClient HTTPClient
	tokenID    string
	endpoint   *url.URL
	userAgent  string
}

func NewClient(endpoint, tokenID, userAgent string, httpClient HTTPClient) (*Client, error) {
	if strings.TrimSpace(endpoint) == "" {
		return nil, &Error{Kind: KindInvalidRequest, Message: "the client endpoint is required"}
	}

	parsedEndpoint, err := url.Parse(endpoint)
	if err != nil {
		return nil, &Error{Kind: KindInvalidRequest, Message: "unable to parse the client endpoint", Err: err}
	}

	if (parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") || parsedEndpoint.Host == "" {
		return nil, &Error{
			Kind:    KindInvalidRequest,
			Message: "the client endpoint must be an absolute HTTP or HTTPS URL",
		}
	}

	if strings.TrimSpace(tokenID) == "" {
		return nil, &Error{Kind: KindInvalidRequest, Message: "the project-scoped token is required"}
	}

	client := &Client{
		endpoint:   parsedEndpoint,
		tokenID:    tokenID,
		userAgent:  userAgent,
		httpClient: httpClient,
	}

	if client.httpClient == nil {
		client.httpClient = newHTTPClient()
	}

	// Custom clients own their redirect policy.
	if standard, ok := client.httpClient.(*http.Client); ok {
		client.httpClient = withoutRedirects(standard)
	}

	client.userAgent = buildUserAgent(client.userAgent)

	return client, nil
}

// withoutRedirects copies the client and prevents credentials from being forwarded by redirects.
func withoutRedirects(httpClient *http.Client) *http.Client {
	copied := *httpClient
	copied.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return &copied
}

type Response struct {
	StatusCode int
	RequestID  string
}

func DoRequest(
	ctx context.Context,
	client *Client,
	method, path string, expectedStatus int,
	requestBody, result any,
	options ...RequestOption,
) (*Response, error) {
	requestOptions := newRequestOptions(options)

	request, err := client.newRequest(ctx, method, path, requestBody, requestOptions)
	if err != nil {
		return nil, err
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, newTransportError(ctx, err)
	}
	defer response.Body.Close()

	return handleResponse(ctx, response, result, expectedStatus, requestOptions)
}

func (client *Client) newRequest(
	ctx context.Context, method, path string, requestBody any, options *requestOptions,
) (*http.Request, error) {
	requestURL, err := client.resolveURL(path, options.query)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader

	if requestBody != nil {
		encoded, marshalErr := json.Marshal(requestBody)
		if marshalErr != nil {
			return nil, &Error{
				Kind:    KindInvalidRequest,
				Message: "unable to encode the request body",
				Err:     marshalErr,
			}
		}

		bodyReader = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, &Error{Kind: KindInvalidRequest, Message: "unable to build the request", Err: err}
	}

	request.Header.Set(headerUserAgent, client.userAgent)
	request.Header.Set(headerAuthToken, client.tokenID)
	request.Header.Set(headerAccept, contentTypeJSON)

	if bodyReader != nil {
		request.Header.Set(headerContentType, contentTypeJSON)
	}

	if options.microversion != "" {
		request.Header.Set(headerAPIVersion, apiVersionService+" "+options.microversion)
	}

	return request, nil
}

// resolveURL anchors absolute pagination links to the configured endpoint. This also prevents a
// bad link from sending the token to another host.
func (client *Client) resolveURL(path string, query url.Values) (string, error) {
	target := client.endpoint.JoinPath(path)

	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		absolute, err := url.Parse(path)
		if err != nil {
			return "", &Error{Kind: KindInvalidRequest, Message: "unable to parse the request URL", Err: err}
		}

		if absolute.Path == "" {
			return "", &Error{Kind: KindInvalidRequest, Message: "the request URL has no path"}
		}

		target = &url.URL{
			Scheme:   client.endpoint.Scheme,
			Host:     client.endpoint.Host,
			Path:     absolute.Path,
			RawQuery: absolute.RawQuery,
		}
	}

	if len(query) != 0 {
		target.RawQuery = mergeQuery(target.Query(), query).Encode()
	}

	return target.String(), nil
}

func mergeQuery(target, extra url.Values) url.Values {
	for key, values := range extra {
		for _, value := range values {
			target.Add(key, value)
		}
	}

	return target
}

func handleResponse(
	ctx context.Context, response *http.Response, result any, expectedStatus int, options *requestOptions,
) (*Response, error) {
	meta := &Response{
		StatusCode: response.StatusCode,
		RequestID:  extractRequestID(response.Header),
	}

	if response.StatusCode != expectedStatus {
		return meta, errorFromResponse(meta, response.Body)
	}

	if result == nil {
		// Ignore drain errors after the API has accepted the operation.
		_, _ = io.Copy(io.Discard, response.Body)

		return meta, nil
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return meta, newBodyReadError(ctx, meta, err)
	}

	if len(bytes.TrimSpace(body)) == 0 {
		return meta, unexpectedResponse(meta, "the response body is empty", nil)
	}

	if err := json.Unmarshal(body, result); err != nil {
		return meta, unexpectedResponse(meta, "unable to decode the response body", err)
	}

	if options.responseEnvelope != "" {
		if err := validateResponseEnvelope(body, options.responseEnvelope); err != nil {
			return meta, unexpectedResponse(meta, err.Error(), nil)
		}
	}

	return meta, nil
}

func validateResponseEnvelope(body []byte, name string) error {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return err
	}

	raw, ok := envelope[name]
	trimmed := bytes.TrimSpace(raw)
	if !ok || len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return &responseEnvelopeError{name: name}
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &object); err == nil && len(object) == 0 {
		return &responseEnvelopeError{name: name}
	}

	return nil
}

type responseEnvelopeError struct {
	name string
}

func (e *responseEnvelopeError) Error() string {
	return "the response body does not contain a non-empty " + e.name + " envelope"
}

func unexpectedResponse(meta *Response, message string, err error) *Error {
	return &Error{
		Kind:       KindUnexpected,
		StatusCode: meta.StatusCode,
		RequestID:  meta.RequestID,
		Message:    message,
		Err:        err,
	}
}

func newHTTPClient() *http.Client {
	return &http.Client{
		Timeout:   defaultHTTPTimeout * time.Second,
		Transport: newHTTPTransport(),
	}
}

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
