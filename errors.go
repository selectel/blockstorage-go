package blockstorage

import (
	"errors"
	"fmt"
)

// Kind is a class of an error returned by the SDK.
//
// The caller is expected to branch on the kind instead of parsing error messages
// or comparing raw HTTP status codes.
type Kind string

const (
	// KindNotFound means that the requested resource does not exist for the current scope.
	// A public Cinder endpoint can also answer with it when the token lacks permissions,
	// so the caller must confirm the absence before acting on it.
	KindNotFound Kind = "not_found"

	// KindForbidden means that the token is not allowed to perform the operation.
	KindForbidden Kind = "forbidden"

	// KindConflict means that the current state of the resource does not allow the operation.
	KindConflict Kind = "conflict"

	// KindInvalidRequest means that the request itself was rejected as malformed or invalid.
	KindInvalidRequest Kind = "invalid_request"

	// KindRateLimited means that the request was throttled by the API.
	KindRateLimited Kind = "rate_limited"

	// KindServerError means that the API answered with a 5xx status code.
	KindServerError Kind = "server_error"

	// KindTransport means that the request has not produced an HTTP response at all.
	KindTransport Kind = "transport"

	// KindTimeout means that the request was cancelled or timed out before a response was received.
	KindTimeout Kind = "timeout"

	// KindMicroversion means that the endpoint has rejected the Cinder microversion
	// required by the operation.
	KindMicroversion Kind = "microversion"

	// KindIncompleteList means that a paginated read has not been finished and the returned
	// collection must not be treated as complete.
	KindIncompleteList Kind = "incomplete_list"

	// KindUnexpected means that the response could not be classified or decoded.
	KindUnexpected Kind = "unexpected"
)

// Error represents an error returned by the Selectel Block Storage API or by the transport.
//
// It keeps every diagnostic detail that is available at the moment of the failure so that
// the caller can report it without issuing additional requests.
type Error struct {
	// Kind is a class of the error.
	Kind Kind

	// StatusCode is an HTTP status code of the response, if there was a response.
	StatusCode int

	// RequestID is a request identifier reported by the API, if it was present in the response.
	RequestID string

	// Message is a safe diagnostic message extracted from the response body or from the transport.
	Message string

	// Err is an underlying error, if the failure was caused by one.
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	details := string(e.Kind)

	if e.StatusCode != 0 {
		details = fmt.Sprintf("%s: status %d", details, e.StatusCode)
	}

	if e.RequestID != "" {
		details = fmt.Sprintf("%s: request id %s", details, e.RequestID)
	}

	if e.Message != "" {
		details = fmt.Sprintf("%s: %s", details, e.Message)
	}

	return "blockstorage: " + details
}

// Unwrap returns the underlying error, if there is one.
func (e *Error) Unwrap() error {
	return e.Err
}

// IsKind reports whether err is an SDK error of the given kind.
func IsKind(err error, kind Kind) bool {
	var sdkErr *Error
	if !errors.As(err, &sdkErr) {
		return false
	}

	return sdkErr.Kind == kind
}
