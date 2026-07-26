package blockstorage

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorKeepsDiagnosticDetails(t *testing.T) {
	err := &Error{
		Kind:       KindForbidden,
		StatusCode: http.StatusForbidden,
		RequestID:  "req-0a1b2c3d",
		Message:    "policy does not allow this operation",
	}

	message := err.Error()

	assert.Contains(t, message, string(KindForbidden))
	assert.Contains(t, message, "403")
	assert.Contains(t, message, "req-0a1b2c3d")
	assert.Contains(t, message, "policy does not allow this operation")
}

func TestErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("connection reset by peer")
	err := &Error{Kind: KindTransport, Err: cause}

	assert.ErrorIs(t, err, cause)
}

func TestErrorIsKind(t *testing.T) {
	err := &Error{Kind: KindNotFound, StatusCode: http.StatusNotFound}

	assert.True(t, IsKind(err, KindNotFound))
	assert.False(t, IsKind(err, KindForbidden))
	assert.False(t, IsKind(errors.New("plain error"), KindNotFound))
}
