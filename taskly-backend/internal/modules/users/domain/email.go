package domain

import (
	"strings"
	"unicode"

	sharedomain "taskly/backend/internal/shared/domain"
)

const emailMaxLength = 254

type Email struct {
	value string
}

func NewEmail(raw sharedomain.Raw) (Email, error) {
	text, ok := raw.Text()
	if !ok {
		return Email{}, sharedomain.NewValidationError("Email is required")
	}
	value := sharedomain.TrimText(text)
	if value == "" || sharedomain.TextLength(value) > emailMaxLength || !hasEmailShape(value) {
		return Email{}, sharedomain.NewValidationError("Email is invalid")
	}
	return Email{value: value}, nil
}

func RestoreEmail(value string) Email {
	return Email{value: value}
}

func (e Email) Value() string {
	return e.value
}

func (e Email) Equals(other Email) bool {
	return e.value == other.value
}

func hasEmailShape(value string) bool {
	if strings.IndexFunc(value, isEmailSpace) >= 0 {
		return false
	}
	if strings.Count(value, "@") != 1 {
		return false
	}
	local, domainPart, _ := strings.Cut(value, "@")
	if local == "" || domainPart == "" {
		return false
	}
	for index := 1; index < len(domainPart)-1; index++ {
		if domainPart[index] == '.' {
			return true
		}
	}
	return false
}

func isEmailSpace(character rune) bool {
	return character == 0xFEFF || unicode.IsSpace(character)
}
