package domain

import "errors"

type ErrorKind string

const (
	KindValidation     ErrorKind = "VALIDATION_ERROR"
	KindNotFound       ErrorKind = "NOT_FOUND"
	KindAlreadyExists  ErrorKind = "ALREADY_EXISTS"
	KindAuthentication ErrorKind = "AUTHENTICATION_FAILED"
	KindForbidden      ErrorKind = "FORBIDDEN"
)

type Error struct {
	Kind    ErrorKind
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func NewValidationError(message string) error {
	return &Error{Kind: KindValidation, Message: message}
}

func NewNotFoundError(message string) error {
	return &Error{Kind: KindNotFound, Message: message}
}

func NewAlreadyExistsError(message string) error {
	return &Error{Kind: KindAlreadyExists, Message: message}
}

func NewAuthenticationError(message string) error {
	return &Error{Kind: KindAuthentication, Message: message}
}

func NewForbiddenError(message string) error {
	return &Error{Kind: KindForbidden, Message: message}
}

func KindOf(err error) (ErrorKind, bool) {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.Kind, true
	}
	return "", false
}

func IsKind(err error, kind ErrorKind) bool {
	actual, ok := KindOf(err)
	return ok && actual == kind
}
