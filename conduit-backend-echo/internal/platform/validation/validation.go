// Package validation wraps go-playground/validator/v10 and renders contract
// messages ("can't be blank", "is too short (minimum is 3 characters)", ...).
package validation

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// Errors collects messages per JSON field name.
type Errors map[string][]string

// Add appends a message for field.
func (e Errors) Add(field, msg string) { e[field] = append(e[field], msg) }

// Has reports whether field already has an error.
func (e Errors) Has(field string) bool { return len(e[field]) > 0 }

// Validator validates single values against validator tags. Using Var (instead
// of struct tags) keeps partial updates simple: only fields that are present
// get validated.
type Validator struct{ v *validator.Validate }

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// New builds a Validator with the custom "username" and "httpsurl" rules.
func New() *Validator {
	v := validator.New()
	_ = v.RegisterValidation("username", func(fl validator.FieldLevel) bool {
		return usernameRe.MatchString(fl.Field().String())
	})
	_ = v.RegisterValidation("httpsurl", func(fl validator.FieldLevel) bool {
		return IsHTTPSURL(fl.Field().String())
	})
	return &Validator{v: v}
}

// IsHTTPSURL reports whether s is an absolute https URL without whitespace.
func IsHTTPSURL(s string) bool {
	if s == "" || strings.IndexFunc(s, unicode.IsSpace) >= 0 {
		return false
	}
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Var validates value against tag and records the first failure under field.
// String length rules (min/max) count Unicode characters, not bytes.
func (v *Validator) Var(errs Errors, field string, value any, tag string) {
	err := v.v.Var(value, tag)
	if err == nil {
		return
	}
	var ves validator.ValidationErrors
	if errors.As(err, &ves) {
		for _, fe := range ves {
			errs.Add(field, Message(fe))
		}
		return
	}
	errs.Add(field, "is invalid")
}

// Message renders a human readable message for a failed rule.
func Message(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "can't be blank"
	case "min":
		return fmt.Sprintf("is too short (minimum is %s characters)", fe.Param())
	case "max":
		return fmt.Sprintf("is too long (maximum is %s characters)", fe.Param())
	default:
		return "is invalid"
	}
}
