package domain

import (
	"strings"
	"testing"
	"time"

	sharedomain "taskly/backend/internal/shared/domain"
)

func message(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestUsername(t *testing.T) {
	if value, err := NewUsername(sharedomain.RawString("  john  ")); err != nil || value.Value() != "john" {
		t.Fatalf("got %q %v", value.Value(), err)
	}
	cases := []struct {
		raw  sharedomain.Raw
		want string
	}{
		{sharedomain.RawOf(1), "Username is required"},
		{sharedomain.Raw{}, "Username is required"},
		{sharedomain.RawString("ab"), "Username must be between 3 and 50 characters"},
		{sharedomain.RawString(strings.Repeat("a", 51)), "Username must be between 3 and 50 characters"},
	}
	for _, tc := range cases {
		if _, err := NewUsername(tc.raw); message(err) != tc.want {
			t.Errorf("got %q, want %q", message(err), tc.want)
		}
	}
}

func TestEmail(t *testing.T) {
	valid := []string{"a@b.co", "  john@mail.com  ", "first.last+tag@sub.example.org"}
	for _, value := range valid {
		if _, err := NewEmail(sharedomain.RawString(value)); err != nil {
			t.Errorf("%q should be valid: %v", value, err)
		}
	}
	invalid := []string{"", "plain", "a@b", "a@@b.co", "@b.co", "a@.co", "a@b.", "a b@c.co", "a@b .co", strings.Repeat("a", 250) + "@b.co"}
	for _, value := range invalid {
		if _, err := NewEmail(sharedomain.RawString(value)); message(err) != "Email is invalid" {
			t.Errorf("%q should be invalid, got %q", value, message(err))
		}
	}
	if _, err := NewEmail(sharedomain.RawOf(nil)); message(err) != "Email is required" {
		t.Errorf("got %q", message(err))
	}
}

func TestPlainPassword(t *testing.T) {
	if _, err := NewPlainPassword(sharedomain.RawString("exactly8")); err != nil {
		t.Fatal(err)
	}
	if _, err := NewPlainPassword(sharedomain.RawString(strings.Repeat("a", 72))); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		raw  sharedomain.Raw
		want string
	}{
		{sharedomain.RawOf(12345678), "Password is required"},
		{sharedomain.RawString("short"), "Password must be at least 8 characters long"},
		{sharedomain.RawString(strings.Repeat("a", 73)), "Password must not exceed 72 bytes"},
		{sharedomain.RawString(strings.Repeat("é", 37)), "Password must not exceed 72 bytes"},
	}
	for _, tc := range cases {
		if _, err := NewPlainPassword(tc.raw); message(err) != tc.want {
			t.Errorf("got %q, want %q", message(err), tc.want)
		}
	}
}

func TestAvatarPaths(t *testing.T) {
	custom := []string{"/images/users/avatar-1-2.png", "/images/users/avatar-a_b.c-d.jpg"}
	for _, value := range custom {
		if !IsCustomAvatar(value) {
			t.Errorf("%q should be custom", value)
		}
	}
	notCustom := []string{DefaultAvatarPath, "", "/images/users/avatar-", "/images/users/avatar-../x", "/etc/passwd", "/images/users/avatar-a\n"}
	for _, value := range notCustom {
		if IsCustomAvatar(value) {
			t.Errorf("%q should not be custom", value)
		}
	}
	if ToAvatarPath("avatar-1.png") != "/images/users/avatar-1.png" {
		t.Error("ToAvatarPath mismatch")
	}
}

func TestUserLifecycle(t *testing.T) {
	username, _ := NewUsername(sharedomain.RawString("john"))
	email, _ := NewEmail(sharedomain.RawString("john@mail.com"))
	now := time.Date(2026, 1, 1, 10, 0, 0, 1234567, time.UTC)

	user := Register(Registration{ID: "id-1", Username: username, Email: email, PasswordHash: "hash", Now: now})
	if user.Avatar() != DefaultAvatarPath || !user.CreatedAt().Equal(user.UpdatedAt()) {
		t.Fatalf("unexpected new user: %+v", user.Snapshot())
	}
	if user.CreatedAt().Nanosecond() != 1234000 {
		t.Fatalf("expected microsecond precision, got %d", user.CreatedAt().Nanosecond())
	}

	newName, _ := NewUsername(sharedomain.RawString("johnny"))
	empty := ""
	later := now.Add(time.Hour)
	user.UpdateProfile(ProfileChanges{Username: &newName, PasswordHash: &empty}, later)
	if user.Username().Value() != "johnny" || user.PasswordHash() != "hash" || !user.UpdatedAt().After(user.CreatedAt()) {
		t.Fatalf("unexpected profile update: %+v", user.Snapshot())
	}

	newHash := "hash-2"
	user.UpdateProfile(ProfileChanges{PasswordHash: &newHash}, later)
	user.ChangeAvatar("/images/users/avatar-1-1.png", later.Add(time.Minute))
	if user.PasswordHash() != "hash-2" || user.Avatar() != "/images/users/avatar-1-1.png" {
		t.Fatalf("unexpected state: %+v", user.Snapshot())
	}

	restored := Restore(user.Snapshot())
	if restored.Snapshot() != user.Snapshot() {
		t.Fatal("restore must round trip")
	}
}
