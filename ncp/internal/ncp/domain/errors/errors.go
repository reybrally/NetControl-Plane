package errors

import (
	"errors"
	"fmt"
	"net/http"
)

type Code string

const (
	CodeUnauthenticated Code = "UNAUTHENTICATED"
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeGuardrailDenied Code = "GUARDRAIL_DENIED"
	CodeInternal        Code = "INTERNAL"
)

type Error struct {
	Code    Code
	Message string
	Details map[string]any
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	return string(e.Code)
}

func (e *Error) Unwrap() error { return e.Cause }

func New(code Code, msg string, details map[string]any, cause error) *Error {
	return &Error{Code: code, Message: msg, Details: details, Cause: cause}
}

func InvalidArgument(msg string, details map[string]any, cause error) *Error {
	return New(CodeInvalidArgument, msg, details, cause)
}

func NotFound(msg string, details map[string]any, cause error) *Error {
	return New(CodeNotFound, msg, details, cause)
}

func Conflict(msg string, details map[string]any, cause error) *Error {
	return New(CodeConflict, msg, details, cause)
}

func GuardrailDenied(msg string, details map[string]any, cause error) *Error {
	return New(CodeGuardrailDenied, msg, details, cause)
}

func Internal(msg string, details map[string]any, cause error) *Error {
	return New(CodeInternal, msg, details, cause)
}

func As(err error) *Error {
	if err == nil {
		return nil
	}
	var e *Error
	if errors.As(err, &e) {
		return e
	}

	return Internal("internal error", nil, err)
}

func HTTPStatus(code Code) int {
	switch code {
	case CodeInvalidArgument:
		return http.StatusBadRequest
	case CodeNotFound:
		return http.StatusNotFound
	case CodeConflict:
		return http.StatusConflict
	case CodeGuardrailDenied:

		return http.StatusUnprocessableEntity
	case CodeUnauthenticated:
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

func Wrapf(code Code, cause error, format string, a ...any) *Error {
	return New(code, fmt.Sprintf(format, a...), nil, cause)
}

func Unauthenticated(msg string, details map[string]any, cause error) *Error {
	return New(CodeUnauthenticated, msg, details, cause)
}
