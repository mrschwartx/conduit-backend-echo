// Package articles implements articles, tags, favorites and the personal feed.
package articles

import (
	"time"

	"conduit/backend/internal/profiles"
)

// Article is the assembled article with author profile and caller-relative flags.
type Article struct {
	ID             int64
	AuthorID       int64
	Slug           string
	Title          string
	Description    string
	Body           string
	TagList        []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Favorited      bool
	FavoritesCount int
	Author         profiles.Profile
}

// Filter narrows article queries; zero values are ignored.
type Filter struct {
	Slug        string
	Tag         string
	Author      string
	FavoritedBy string
	FeedFor     int64 // only articles by authors this user follows
}

// NewRecord is the data persisted for a new article.
type NewRecord struct {
	AuthorID    int64
	Slug        string
	Title       string
	Description string
	Body        string
	Tags        []string
	Now         time.Time
}

// UpdateRecord carries only the changed fields (nil = unchanged).
type UpdateRecord struct {
	Slug        *string
	Title       *string
	Description *string
	Body        *string
	Tags        *[]string
	Now         time.Time
}
