package transport

import "net/url"

type requestOptions struct {
	microversion     string
	query            url.Values
	responseEnvelope string
}

type RequestOption func(*requestOptions)

func newRequestOptions(options []RequestOption) *requestOptions {
	result := &requestOptions{}

	for _, option := range options {
		option(result)
	}

	return result
}

func WithMicroversion(microversion string) RequestOption {
	return func(options *requestOptions) {
		options.microversion = microversion
	}
}

func WithQuery(query url.Values) RequestOption {
	return func(options *requestOptions) {
		options.query = query
	}
}

func WithResponseEnvelope(name string) RequestOption {
	return func(options *requestOptions) {
		options.responseEnvelope = name
	}
}
