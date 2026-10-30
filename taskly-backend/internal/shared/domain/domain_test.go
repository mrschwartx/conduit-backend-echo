package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewEntityIDProducesLowercaseV4UUIDs(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		id := NewEntityID()
		if !IsEntityID(id) || id != strings.ToLower(id) || id[14] != '4' {
			t.Fatalf("invalid id %q", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestParseEntityID(t *testing.T) {
	id, err := ParseEntityID("AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "user")
	if err != nil || id != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("got %q %v", id, err)
	}

	invalid := []string{"", "not-an-id", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa", "aaaaaaaaaaaa4aaa8aaaaaaaaaaaaaaa",
		"urn:uuid:aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "gggggggg-gggg-4ggg-8ggg-gggggggggggg"}
	for _, value := range invalid {
		_, err := ParseEntityID(value, "task")
		if !IsKind(err, KindValidation) || err.Error() != "Invalid task ID format" {
			t.Errorf("ParseEntityID(%q) = %v", value, err)
		}
	}
}

func TestErrorKinds(t *testing.T) {
	cases := map[ErrorKind]error{
		KindValidation:     NewValidationError("a"),
		KindNotFound:       NewNotFoundError("a"),
		KindAlreadyExists:  NewAlreadyExistsError("a"),
		KindAuthentication: NewAuthenticationError("a"),
		KindForbidden:      NewForbiddenError("a"),
	}
	for kind, err := range cases {
		if !IsKind(err, kind) || err.Error() != "a" {
			t.Errorf("kind %s mismatch for %v", kind, err)
		}
		wrapped := fmt.Errorf("wrapped: %w", err)
		if got, ok := KindOf(wrapped); !ok || got != kind {
			t.Errorf("wrapped kind %s mismatch", kind)
		}
	}
	if _, ok := KindOf(errors.New("plain")); ok {
		t.Error("plain errors have no kind")
	}
}

func TestRawDistinguishesAbsentNullAndValues(t *testing.T) {
	var payload struct {
		Present Raw `json:"present"`
		Null    Raw `json:"null"`
		Number  Raw `json:"number"`
		Missing Raw `json:"missing"`
	}
	if err := json.Unmarshal([]byte(`{"present":"hi","null":null,"number":5}`), &payload); err != nil {
		t.Fatal(err)
	}
	if text, ok := payload.Present.Text(); !ok || text != "hi" {
		t.Errorf("present = %q %v", text, ok)
	}
	if !payload.Null.IsSet() {
		t.Error("null must count as set")
	}
	if _, ok := payload.Null.Text(); ok {
		t.Error("null is not text")
	}
	if !payload.Number.IsSet() {
		t.Error("number must count as set")
	}
	if _, ok := payload.Number.Text(); ok {
		t.Error("number is not text")
	}
	if payload.Missing.IsSet() {
		t.Error("missing must not count as set")
	}
}

func TestTextRuleParse(t *testing.T) {
	empty := ""
	rule := TextRule{Label: "Thing", MinLength: 2, MaxLength: 5, AllowedValues: []string{"ab", "abc"}, Default: &empty}

	if value, err := rule.Parse(RawString("  ab ")); err != nil || value != "ab" {
		t.Fatalf("trim failed: %q %v", value, err)
	}
	expectations := []struct {
		raw  Raw
		want string
	}{
		{RawOf(12), "Thing is required"},
		{RawOf(nil), "Thing is required"},
		{RawString("a"), "Thing must be between 2 and 5 characters"},
		{RawString("abcdef"), "Thing must be between 2 and 5 characters"},
		{RawString("zz"), "Thing must be one of: ab, abc"},
		{Raw{}, "Thing must be between 2 and 5 characters"},
	}
	for _, tc := range expectations {
		_, err := rule.Parse(tc.raw)
		if !IsKind(err, KindValidation) || err.Error() != tc.want {
			t.Errorf("Parse(%v) = %v, want %q", tc.raw, err, tc.want)
		}
	}

	if _, err := (TextRule{Label: "Name", MinLength: 1, MaxLength: 3}).Parse(Raw{}); err == nil || err.Error() != "Name is required" {
		t.Errorf("missing without default must be required, got %v", err)
	}
}

func TestTextLengthCountsUTF16Units(t *testing.T) {
	cases := map[string]int{"abc": 3, "é": 1, "😀": 2, "日本語": 3}
	for input, want := range cases {
		if got := TextLength(input); got != want {
			t.Errorf("TextLength(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestTrimText(t *testing.T) {
	if got := TrimText("\u00a0\ufeff  hello \n\t"); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestTimestampHelpers(t *testing.T) {
	value := time.Date(2026, 3, 4, 5, 6, 7, 123456789, time.FixedZone("x", 3600))
	if got := FormatTimestamp(value); got != "2026-03-04T04:06:07.123Z" {
		t.Errorf("format = %s", got)
	}
	if got := TruncateToMillisecond(value).Nanosecond(); got != 123000000 {
		t.Errorf("ms truncation = %d", got)
	}
	if got := TruncateToMicrosecond(value).Nanosecond(); got != 123456000 {
		t.Errorf("µs truncation = %d", got)
	}
}
