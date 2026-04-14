package service

import (
	"errors"
	"net/http"
)

type HandlerResult = any

type HTTPStatusError struct {
	Status int
	Err    error
}

func (e *HTTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	if e.Status > 0 {
		return http.StatusText(e.Status)
	}
	return ""
}

func (e *HTTPStatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func WithStatus(status int, err error) error {
	if err == nil {
		return nil
	}
	return &HTTPStatusError{Status: status, Err: err}
}

func StatusCode(err error) int {
	if err == nil {
		return http.StatusOK
	}
	var statusErr *HTTPStatusError
	if errors.As(err, &statusErr) && statusErr.Status > 0 {
		return statusErr.Status
	}
	return http.StatusInternalServerError
}
