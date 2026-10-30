package domain

import (
	"fmt"

	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	passwordMinLength = 8
	passwordMaxBytes  = 72
)

type PlainPassword struct {
	value string
}

func NewPlainPassword(raw sharedomain.Raw) (PlainPassword, error) {
	text, ok := raw.Text()
	if !ok {
		return PlainPassword{}, sharedomain.NewValidationError("Password is required")
	}
	if sharedomain.TextLength(text) < passwordMinLength {
		return PlainPassword{}, sharedomain.NewValidationError(
			fmt.Sprintf("Password must be at least %d characters long", passwordMinLength),
		)
	}
	if len(text) > passwordMaxBytes {
		return PlainPassword{}, sharedomain.NewValidationError(
			fmt.Sprintf("Password must not exceed %d bytes", passwordMaxBytes),
		)
	}
	return PlainPassword{value: text}, nil
}

func (p PlainPassword) Value() string {
	return p.value
}
