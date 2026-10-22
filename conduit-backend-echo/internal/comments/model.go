// Package comments implements article comments.
package comments

import (
	"time"

	"conduit/backend/internal/profiles"
)

// Comment is a comment with its author profile (relative to the caller).
type Comment struct {
	ID        int64
	ArticleID int64
	AuthorID  int64
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    profiles.Profile
}
