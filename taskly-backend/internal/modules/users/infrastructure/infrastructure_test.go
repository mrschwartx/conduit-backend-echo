package infrastructure

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	secret = "test-secret-value-with-more-than-32-chars"
	userID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

func TestBcryptHasher(t *testing.T) {
	hasher := NewBcryptHasher(4)
	ctx := context.Background()

	hash, err := hasher.Hash(ctx, "Password123!")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "Password123!" || !strings.HasPrefix(hash, "$2a$04$") {
		t.Fatalf("unexpected hash %q", hash)
	}
	if !hasher.Compare(ctx, "Password123!", hash) || hasher.Compare(ctx, "other", hash) {
		t.Fatal("compare mismatch")
	}
	if hasher.Compare(ctx, "x", "not-a-hash") {
		t.Fatal("malformed hashes must not match")
	}
	second, _ := hasher.Hash(ctx, "Password123!")
	if second == hash {
		t.Fatal("hashes must be salted")
	}
}

func TestJWTTokenService(t *testing.T) {
	service := NewJWTTokenService(secret, time.Hour)

	token, err := service.Issue(userID)
	if err != nil {
		t.Fatal(err)
	}
	if id, err := service.Verify(token); err != nil || id != userID {
		t.Fatalf("verify failed: %q %v", id, err)
	}

	upper := NewJWTTokenService(secret, time.Hour)
	upperToken, _ := upper.Issue(strings.ToUpper(userID))
	if id, err := service.Verify(upperToken); err != nil || id != userID {
		t.Fatalf("ids must be lowercased: %q %v", id, err)
	}

	if _, err := NewJWTTokenService("another-secret-with-more-than-32-characters", time.Hour).Verify(token); err == nil {
		t.Fatal("tokens signed with another secret must be rejected")
	}
	if _, err := service.Verify("garbage"); err == nil {
		t.Fatal("garbage must be rejected")
	}
}

func TestJWTTokenServiceRejectsUnsafeTokens(t *testing.T) {
	service := NewJWTTokenService(secret, time.Hour)

	expired := NewJWTTokenService(secret, -time.Minute)
	expiredToken, _ := expired.Issue(userID)
	if _, err := service.Verify(expiredToken); err == nil {
		t.Fatal("expired tokens must be rejected")
	}

	none, _ := jwt.NewWithClaims(jwt.SigningMethodNone, tokenClaims{ID: userID}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	if _, err := service.Verify(none); err == nil {
		t.Fatal("alg none must be rejected")
	}

	hs512, _ := jwt.NewWithClaims(jwt.SigningMethodHS512, tokenClaims{
		ID:               userID,
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if _, err := service.Verify(hs512); err == nil {
		t.Fatal("other HMAC algorithms must be rejected")
	}

	badPayload, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{
		ID:               "not-a-uuid",
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}).SignedString([]byte(secret))
	if _, err := service.Verify(badPayload); err == nil || err.Error() != "Invalid token payload" {
		t.Fatalf("expected payload error, got %v", err)
	}

	noExpiry, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims{ID: userID}).SignedString([]byte(secret))
	if _, err := service.Verify(noExpiry); err == nil {
		t.Fatal("tokens without expiry must be rejected")
	}
}

func TestLocalAvatarStorage(t *testing.T) {
	publicDir := t.TempDir()
	avatarsDir := filepath.Join(publicDir, "images", "users")
	if err := os.MkdirAll(avatarsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	storage := NewLocalAvatarStorage(publicDir, slog.New(slog.NewTextHandler(&logs, nil)))
	ctx := context.Background()

	custom := filepath.Join(avatarsDir, "avatar-1-1.png")
	protected := filepath.Join(avatarsDir, "default-avatar.jpg")
	for _, file := range []string{custom, protected} {
		if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	storage.Remove(ctx, "/images/users/avatar-1-1.png")
	storage.Remove(ctx, "/images/users/default-avatar.jpg")
	storage.Remove(ctx, "/images/users/avatar-missing.png")
	storage.Remove(ctx, "/images/users/../../secret")
	storage.Remove(ctx, "")

	if _, err := os.Stat(custom); !os.IsNotExist(err) {
		t.Fatal("custom avatar should be removed")
	}
	if _, err := os.Stat(protected); err != nil {
		t.Fatal("default avatar must never be removed")
	}
	if logs.Len() != 0 {
		t.Fatalf("no warnings expected, got %s", logs.String())
	}
}

func TestLocalAvatarStorageWarnsWhenRemovalFails(t *testing.T) {
	publicDir := t.TempDir()
	directory := filepath.Join(publicDir, "images", "users", "avatar-directory-1")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	storage := NewLocalAvatarStorage(publicDir, slog.New(slog.NewTextHandler(&logs, nil)))

	storage.Remove(context.Background(), "/images/users/avatar-directory-1")

	if !strings.Contains(logs.String(), "Failed to remove avatar file") {
		t.Fatalf("expected a warning, got %q", logs.String())
	}
	if _, err := os.Stat(directory); err != nil {
		t.Fatal("directories must not be removed")
	}
}
