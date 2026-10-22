// Package auth issues and verifies HS256 JWTs and provides the Echo middleware
// for the "Authorization: Token <jwt>" scheme (NOT Bearer).
package auth

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/httpx"
)

// Sentinel errors returned by Parse.
var (
	ErrInvalid = errors.New("token is invalid")
	ErrExpired = errors.New("token has expired")
)

type claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Service signs and verifies tokens.
type Service struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

// NewService creates a token service.
func NewService(secret string, ttl time.Duration) *Service {
	return &Service{secret: []byte(secret), ttl: ttl, now: time.Now}
}

// Issue creates a signed token with claims sub (user id), username, iat, exp.
func (s *Service) Issue(userID int64, username string) (string, error) {
	now := s.now().UTC()
	c := claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(userID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString(s.secret)
}

// Parse verifies signature, algorithm and expiry and returns the user id.
func (s *Service) Parse(raw string) (int64, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, ErrExpired
		}
		return 0, ErrInvalid
	}
	id, perr := strconv.ParseInt(c.Subject, 10, 64)
	if perr != nil || id <= 0 {
		return 0, ErrInvalid
	}
	return id, nil
}

const userIDKey = "conduit.userID"

// UserID returns the authenticated user id, or 0 for anonymous requests.
func UserID(c *echo.Context) int64 {
	if v, ok := c.Get(userIDKey).(int64); ok {
		return v
	}
	return 0
}

// Required rejects requests without a valid token (401).
func Required(s *Service) echo.MiddlewareFunc { return middleware(s, true) }

// Optional lets anonymous requests through, but a token that is present and
// invalid or expired is still a 401.
func Optional(s *Service) echo.MiddlewareFunc { return middleware(s, false) }

func middleware(s *Service, required bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" {
				if required {
					return httpx.Unauthorized("token", "is missing")
				}
				return next(c)
			}
			scheme, raw, ok := strings.Cut(header, " ")
			raw = strings.TrimSpace(raw)
			if !ok || scheme != "Token" || raw == "" {
				return httpx.Unauthorized("token", "is invalid")
			}
			id, err := s.Parse(raw)
			if err != nil {
				if errors.Is(err, ErrExpired) {
					return httpx.Unauthorized("token", "has expired")
				}
				return httpx.Unauthorized("token", "is invalid")
			}
			c.Set(userIDKey, id)
			return next(c)
		}
	}
}
