// Package identity implements registration, login and the current-user endpoints.
package identity

import "time"

// User is the persisted account.
type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Bio          *string
	Image        *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AuthResult is a user together with a freshly issued JWT.
type AuthResult struct {
	User  *User
	Token string
}
