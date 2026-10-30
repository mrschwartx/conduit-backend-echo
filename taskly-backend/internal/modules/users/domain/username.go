package domain

import (
	"fmt"

	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	usernameMinLength = 3
	usernameMaxLength = 50
)

type Username struct {
	value string
}

func NewUsername(raw sharedomain.Raw) (Username, error) {
	text, ok := raw.Text()
	if !ok {
		return Username{}, sharedomain.NewValidationError("Username is required")
	}
	value := sharedomain.TrimText(text)
	length := sharedomain.TextLength(value)
	if length < usernameMinLength || length > usernameMaxLength {
		return Username{}, sharedomain.NewValidationError(fmt.Sprintf(
			"Username must be between %d and %d characters",
			usernameMinLength, usernameMaxLength,
		))
	}
	return Username{value: value}, nil
}

func RestoreUsername(value string) Username {
	return Username{value: value}
}

func (u Username) Value() string {
	return u.value
}

func (u Username) Equals(other Username) bool {
	return u.value == other.value
}
