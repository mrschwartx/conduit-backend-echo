package identity

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"conduit/backend/internal/platform/database"
)

// Duplicate-key sentinels (MySQL error 1062 on the unique indexes).
var (
	ErrDuplicateEmail    = errors.New("duplicate email")
	ErrDuplicateUsername = errors.New("duplicate username")
)

// Repository contains only SQL for users.
type Repository struct{ db *sql.DB }

// NewRepository creates the repository.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const userColumns = `id, username, email, password_hash, bio, image, created_at, updated_at`

func scanUser(row *sql.Row) (*User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Bio, &u.Image, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func mapDuplicate(err error) error {
	if msg, ok := database.IsDuplicate(err); ok {
		switch {
		case strings.Contains(msg, "uq_users_email"):
			return ErrDuplicateEmail
		case strings.Contains(msg, "uq_users_username"):
			return ErrDuplicateUsername
		}
	}
	return err
}

// Create inserts the user and sets u.ID.
func (r *Repository) Create(ctx context.Context, u *User) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (username, email, password_hash, bio, image, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.Username, u.Email, u.PasswordHash, u.Bio, u.Image, u.CreatedAt.UTC(), u.UpdatedAt.UTC())
	if err != nil {
		return mapDuplicate(err)
	}
	u.ID, err = res.LastInsertId()
	return err
}

// GetByID loads a user by id.
func (r *Repository) GetByID(ctx context.Context, id int64) (*User, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE id = ?`, id))
}

// GetByEmail loads a user by email (case-insensitive via collation).
func (r *Repository) GetByEmail(ctx context.Context, email string) (*User, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE email = ?`, email))
}

// GetByUsername loads a user by username (case-insensitive via collation).
func (r *Repository) GetByUsername(ctx context.Context, username string) (*User, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	return scanUser(r.db.QueryRowContext(ctx, `SELECT `+userColumns+` FROM users WHERE username = ?`, username))
}

// Update persists all mutable columns of u.
func (r *Repository) Update(ctx context.Context, u *User) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET username = ?, email = ?, password_hash = ?, bio = ?, image = ?, updated_at = ?
		 WHERE id = ?`,
		u.Username, u.Email, u.PasswordHash, u.Bio, u.Image, u.UpdatedAt.UTC(), u.ID)
	return mapDuplicate(err)
}
