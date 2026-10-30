package domain

import (
	"time"

	sharedomain "taskly/backend/internal/shared/domain"
)

type User struct {
	id           string
	username     Username
	email        Email
	passwordHash string
	avatar       string
	createdAt    time.Time
	updatedAt    time.Time
}

type Snapshot struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	Avatar       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Registration struct {
	ID           string
	Username     Username
	Email        Email
	PasswordHash string
	Now          time.Time
}

type ProfileChanges struct {
	Username     *Username
	Email        *Email
	PasswordHash *string
}

func Register(registration Registration) User {
	timestamp := sharedomain.TruncateToMicrosecond(registration.Now)
	return User{
		id:           registration.ID,
		username:     registration.Username,
		email:        registration.Email,
		passwordHash: registration.PasswordHash,
		avatar:       DefaultAvatarPath,
		createdAt:    timestamp,
		updatedAt:    timestamp,
	}
}

func Restore(snapshot Snapshot) User {
	return User{
		id:           snapshot.ID,
		username:     RestoreUsername(snapshot.Username),
		email:        RestoreEmail(snapshot.Email),
		passwordHash: snapshot.PasswordHash,
		avatar:       snapshot.Avatar,
		createdAt:    snapshot.CreatedAt.UTC(),
		updatedAt:    snapshot.UpdatedAt.UTC(),
	}
}

func (u User) ID() string {
	return u.id
}

func (u User) Username() Username {
	return u.username
}

func (u User) Email() Email {
	return u.email
}

func (u User) PasswordHash() string {
	return u.passwordHash
}

func (u User) Avatar() string {
	return u.avatar
}

func (u User) CreatedAt() time.Time {
	return u.createdAt
}

func (u User) UpdatedAt() time.Time {
	return u.updatedAt
}

func (u *User) UpdateProfile(changes ProfileChanges, now time.Time) {
	if changes.Username != nil {
		u.username = *changes.Username
	}
	if changes.Email != nil {
		u.email = *changes.Email
	}
	if changes.PasswordHash != nil && *changes.PasswordHash != "" {
		u.passwordHash = *changes.PasswordHash
	}
	u.touch(now)
}

func (u *User) ChangeAvatar(avatar string, now time.Time) {
	u.avatar = avatar
	u.touch(now)
}

func (u User) Snapshot() Snapshot {
	return Snapshot{
		ID:           u.id,
		Username:     u.username.Value(),
		Email:        u.email.Value(),
		PasswordHash: u.passwordHash,
		Avatar:       u.avatar,
		CreatedAt:    u.createdAt,
		UpdatedAt:    u.updatedAt,
	}
}

func (u *User) touch(now time.Time) {
	u.updatedAt = sharedomain.TruncateToMicrosecond(now)
}
