package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const secret = "0123456789abcdef0123456789abcdef"

func TestIssueAndParse(t *testing.T) {
	s := NewService(secret, time.Hour)
	tok, err := s.Issue(42, "alice")
	require.NoError(t, err)

	id, err := s.Parse(tok)
	require.NoError(t, err)
	assert.EqualValues(t, 42, id)
}

func TestParseRejects(t *testing.T) {
	s := NewService(secret, time.Hour)

	expired, err := NewService(secret, -time.Minute).Issue(1, "a")
	require.NoError(t, err)
	_, err = s.Parse(expired)
	assert.ErrorIs(t, err, ErrExpired)

	other, err := NewService("another-secret-another-secret-123456", time.Hour).Issue(1, "a")
	require.NoError(t, err)
	_, err = s.Parse(other)
	assert.ErrorIs(t, err, ErrInvalid)

	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = s.Parse(none)
	assert.ErrorIs(t, err, ErrInvalid, "alg=none must be rejected")

	_, err = s.Parse("garbage")
	assert.ErrorIs(t, err, ErrInvalid)
}
