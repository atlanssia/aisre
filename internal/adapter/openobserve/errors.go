package openobserve

import "fmt"

// AdapterError represents a normalized error from the OpenObserve adapter.
type AdapterError struct {
	Code       string // business error code
	HTTPStatus int
	Message    string
	Retryable  bool
}

func (e *AdapterError) Error() string {
	return fmt.Sprintf("oo-adapter: [%s] %s", e.Code, e.Message)
}

// Error codes
const (
	ErrCodeInvalidQuery    = "INVALID_QUERY"
	ErrCodeAuthFailed      = "AUTH_FAILED"
	ErrCodeStreamNotFound  = "STREAM_NOT_FOUND"
	ErrCodeProviderInternal = "PROVIDER_INTERNAL"
	ErrCodeProviderTimeout = "PROVIDER_TIMEOUT"
)

// Common adapter errors
var (
	ErrInvalidQuery    = &AdapterError{Code: ErrCodeInvalidQuery, HTTPStatus: 400, Retryable: false}
	ErrAuthFailed      = &AdapterError{Code: ErrCodeAuthFailed, HTTPStatus: 401, Retryable: false}
	ErrStreamNotFound  = &AdapterError{Code: ErrCodeStreamNotFound, HTTPStatus: 404, Retryable: false}
	ErrProviderInternal = &AdapterError{Code: ErrCodeProviderInternal, HTTPStatus: 500, Retryable: true}
	ErrProviderTimeout = &AdapterError{Code: ErrCodeProviderTimeout, Message: "request timed out", Retryable: true}
)
