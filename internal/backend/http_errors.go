package backend

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

func classifyHTTPError(backend, operation string, err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return NewBackendError(ErrTimeout, backend, operation, true, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return NewBackendError(ErrTimeout, backend, operation, true, err)
	}
	return NewBackendError(ErrUpstream, backend, operation, true, err)
}

func mapStatusError(backend, operation string, status int) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return NewBackendError(ErrAuth, backend, operation, false, fmt.Errorf("status %d", status))
	case status == http.StatusTooManyRequests:
		return NewBackendError(ErrRateLimit, backend, operation, true, fmt.Errorf("status %d", status))
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		return NewBackendError(ErrTimeout, backend, operation, true, fmt.Errorf("status %d", status))
	case status >= 500:
		return NewBackendError(ErrUpstream, backend, operation, true, fmt.Errorf("status %d", status))
	default:
		return NewBackendError(ErrUpstream, backend, operation, false, fmt.Errorf("status %d", status))
	}
}
