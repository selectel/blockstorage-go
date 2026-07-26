package blockstorage

import (
	"io"
	"net/http"
	"strings"
)

// roundTripFunc lets a plain function be used as an http.RoundTripper.
type roundTripFunc func(request *http.Request) (*http.Response, error)

// RoundTrip implements the http.RoundTripper interface.
func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

// newFakeResponse creates a fake HTTP response with the provided status code and body.
func newFakeResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// newFakeHTTPClient returns an HTTP client that answers every request with the given handler
// and records the requests it has seen.
func newFakeHTTPClient(handler func(request *http.Request) (*http.Response, error)) (*http.Client, *[]*http.Request) {
	seen := make([]*http.Request, 0)

	client := &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			seen = append(seen, request)

			return handler(request)
		}),
	}

	return client, &seen
}
