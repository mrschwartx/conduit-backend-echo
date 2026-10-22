package profiles

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"conduit/backend/internal/platform/database"
)

// Repository contains only SQL for profiles and follows.
type Repository struct{ db *sql.DB }

// NewRepository creates the repository.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// GetByUsername loads a profile; Following is computed for viewerID (0 = anonymous).
func (r *Repository) GetByUsername(ctx context.Context, viewerID int64, username string) (*Profile, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()

	var p Profile
	err := r.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.bio, u.image,
		       EXISTS(SELECT 1 FROM follows f WHERE f.follower_id = ? AND f.followee_id = u.id)
		FROM users u
		WHERE u.username = ?`, viewerID, username).
		Scan(&p.ID, &p.Username, &p.Bio, &p.Image, &p.Following)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// Follow is idempotent.
func (r *Repository) Follow(ctx context.Context, followerID, followeeID int64, now time.Time) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO follows (follower_id, followee_id, created_at) VALUES (?, ?, ?)`,
		followerID, followeeID, now.UTC())
	return err
}

// Unfollow is idempotent.
func (r *Repository) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM follows WHERE follower_id = ? AND followee_id = ?`, followerID, followeeID)
	return err
}
