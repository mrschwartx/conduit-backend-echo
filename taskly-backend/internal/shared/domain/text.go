package domain

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf16"
)

type TextRule struct {
	Label         string
	MinLength     int
	MaxLength     int
	Default       *string
	AllowedValues []string
}

func (rule TextRule) Parse(raw Raw) (string, error) {
	candidate := raw
	if !raw.IsSet() && rule.Default != nil {
		candidate = RawString(*rule.Default)
	}
	text, ok := candidate.Text()
	if !ok {
		return "", NewValidationError(rule.Label + " is required")
	}
	value := TrimText(text)
	length := TextLength(value)
	if length < rule.MinLength || length > rule.MaxLength {
		return "", NewValidationError(fmt.Sprintf(
			"%s must be between %d and %d characters",
			rule.Label, rule.MinLength, rule.MaxLength,
		))
	}
	if len(rule.AllowedValues) > 0 && !slices.Contains(rule.AllowedValues, value) {
		return "", NewValidationError(fmt.Sprintf(
			"%s must be one of: %s",
			rule.Label, strings.Join(rule.AllowedValues, ", "),
		))
	}
	return value, nil
}

func TrimText(value string) string {
	return strings.TrimFunc(value, func(character rune) bool {
		if character == 0xFEFF {
			return true
		}
		return character != 0x85 && unicode.IsSpace(character)
	})
}

func TextLength(value string) int {
	length := 0
	for _, character := range value {
		length += len(utf16.Encode([]rune{character}))
	}
	return length
}
