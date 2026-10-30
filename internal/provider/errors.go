package provider

import "errors"

type ErrorKind string

const (
	ErrorRequest         ErrorKind = "request"
	ErrorAuthentication  ErrorKind = "authentication"
	ErrorRateLimited     ErrorKind = "rate_limited"
	ErrorUnavailable     ErrorKind = "unavailable"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorInvalidResponse ErrorKind = "invalid_response"
)

type Error struct {
	Provider   string
	Kind       ErrorKind
	StatusCode int
	Err        error
}

func (e *Error) Error() string { return e.Err.Error() }
func (e *Error) Unwrap() error { return e.Err }

func Kind(err error) ErrorKind {
	var providerError *Error
	if errors.As(err, &providerError) {
		return providerError.Kind
	}
	return ""
}

func StatusCode(err error) int {
	var providerError *Error
	if errors.As(err, &providerError) {
		return providerError.StatusCode
	}
	return 0
}

func NewError(provider string, kind ErrorKind, statusCode int, err error) error {
	return &Error{Provider: provider, Kind: kind, StatusCode: statusCode, Err: err}
}
