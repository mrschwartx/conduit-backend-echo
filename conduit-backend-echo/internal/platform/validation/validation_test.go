package validation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVar(t *testing.T) {
	v := New()
	tests := []struct {
		name  string
		value string
		tag   string
		want  string // "" = valid
	}{
		{"required blank", "", "required", "can't be blank"},
		{"username ok", "john_doe-1", "required,min=3,max=30,username", ""},
		{"username short", "ab", "required,min=3,max=30,username", "is too short (minimum is 3 characters)"},
		{"username long", strings.Repeat("a", 31), "required,min=3,max=30,username", "is too long (maximum is 30 characters)"},
		{"username chars", "john doe", "required,min=3,max=30,username", "is invalid"},
		{"email ok", "a@example.com", "required,email", ""},
		{"email bad", "nope", "required,email", "is invalid"},
		{"max counts characters not bytes", strings.Repeat("é", 30), "max=30", ""},
		{"image empty ok", "", "omitempty,httpsurl", ""},
		{"image https ok", "https://example.com/a.png", "omitempty,httpsurl", ""},
		{"image http rejected", "http://example.com/a.png", "omitempty,httpsurl", "is invalid"},
		{"image garbage", "not a url", "omitempty,httpsurl", "is invalid"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := Errors{}
			v.Var(errs, "f", tt.value, tt.tag)
			if tt.want == "" {
				assert.Empty(t, errs)
				return
			}
			assert.Equal(t, []string{tt.want}, errs["f"])
		})
	}
}
