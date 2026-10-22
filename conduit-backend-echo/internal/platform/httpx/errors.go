// Package httpx contains the HTTP plumbing shared by every module: the contract
// error format, JSON binding, partial-update types and net/http middleware.
package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/logger"
)

// AppError is an error that maps 1:1 onto the contract error response:
// {"errors": {"<field-or-resource>": ["message", ...]}}.
type AppError struct {
	Status int
	Errors map[string][]string
	Cause  error
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("http %d: %v", e.Status, e.Errors)
}

func (e *AppError) Unwrap() error { return e.Cause }

func single(status int, field, msg string) *AppError {
	return &AppError{Status: status, Errors: map[string][]string{field: {msg}}}
}

// BadRequest is a 400 (malformed body).
func BadRequest(field, msg string) *AppError { return single(http.StatusBadRequest, field, msg) }

// Unauthorized is a 401.
func Unauthorized(field, msg string) *AppError { return single(http.StatusUnauthorized, field, msg) }

// Forbidden is a 403 keyed by the resource type ("article", "comment").
func Forbidden(resource string) *AppError { return single(http.StatusForbidden, resource, "forbidden") }

// NotFound is a 404 keyed by the resource type ("article", "profile", "comment").
func NotFound(resource string) *AppError { return single(http.StatusNotFound, resource, "not found") }

// UnsupportedMediaType is a 415.
func UnsupportedMediaType() *AppError {
	return single(http.StatusUnsupportedMediaType, "body", "content type must be application/json")
}

// PayloadTooLarge is a 413 (body limit exceeded).
func PayloadTooLarge() *AppError {
	return single(http.StatusRequestEntityTooLarge, "body", "is too large")
}

// Conflict is a 409 with all conflicting fields.
func Conflict(errs map[string][]string) *AppError {
	return &AppError{Status: http.StatusConflict, Errors: errs}
}

// Validation is a 422 with all failing fields.
func Validation(errs map[string][]string) *AppError {
	return &AppError{Status: http.StatusUnprocessableEntity, Errors: errs}
}

// ValidationField is a 422 for a single field.
func ValidationField(field, msg string) *AppError {
	return single(http.StatusUnprocessableEntity, field, msg)
}

// Resolve converts any error into a status code and contract error map.
func Resolve(err error) (int, map[string][]string) {
	var app *AppError
	if errors.As(err, &app) {
		return app.Status, app.Errors
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		switch {
		case he.Code == http.StatusNotFound:
			return http.StatusNotFound, map[string][]string{"route": {"not found"}}
		case he.Code == http.StatusMethodNotAllowed:
			return http.StatusMethodNotAllowed, map[string][]string{"route": {"method not allowed"}}
		case he.Code >= 400 && he.Code < 500:
			return he.Code, map[string][]string{"request": {strings.ToLower(http.StatusText(he.Code))}}
		}
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return http.StatusRequestEntityTooLarge, map[string][]string{"body": {"is too large"}}
	}
	return http.StatusInternalServerError, map[string][]string{"server": {"internal server error"}}
}

// ErrorHandler is installed as Echo's HTTPErrorHandler. It renders ONLY the
// contract error format and never leaks internals.
func ErrorHandler() func(c *echo.Context, err error) {
	return func(c *echo.Context, err error) {
		status, errs := Resolve(err)
		if status >= http.StatusInternalServerError {
			logger.FromContext(c.Request().Context()).Error("unexpected error",
				"error", err.Error(), "stack", string(debug.Stack()))
		}
		WriteErrors(c.Response(), status, errs)
	}
}
