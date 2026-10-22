package profiles

import (
	"context"
	"errors"
	"time"

	"conduit/backend/internal/platform/database"
	"conduit/backend/internal/platform/httpx"
)

type repository interface {
	GetByUsername(ctx context.Context, viewerID int64, username string) (*Profile, error)
	Follow(ctx context.Context, followerID, followeeID int64, now time.Time) error
	Unfollow(ctx context.Context, followerID, followeeID int64) error
}

// Service holds the profile business rules.
type Service struct {
	repo repository
	now  func() time.Time
}

// NewService creates the service.
func NewService(repo repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) load(ctx context.Context, viewerID int64, username string) (*Profile, error) {
	p, err := s.repo.GetByUsername(ctx, viewerID, username)
	if errors.Is(err, database.ErrNotFound) {
		return nil, httpx.NotFound("profile")
	}
	return p, err
}

// Get returns a profile (viewerID 0 = anonymous).
func (s *Service) Get(ctx context.Context, viewerID int64, username string) (*Profile, error) {
	return s.load(ctx, viewerID, username)
}

// Follow is idempotent and returns the updated profile.
func (s *Service) Follow(ctx context.Context, viewerID int64, username string) (*Profile, error) {
	target, err := s.load(ctx, viewerID, username)
	if err != nil {
		return nil, err
	}
	if target.ID == viewerID {
		return nil, httpx.ValidationField("username", "you cannot follow yourself")
	}
	if err := s.repo.Follow(ctx, viewerID, target.ID, s.now()); err != nil {
		return nil, err
	}
	target.Following = true
	return target, nil
}

// Unfollow is idempotent and returns the updated profile.
func (s *Service) Unfollow(ctx context.Context, viewerID int64, username string) (*Profile, error) {
	target, err := s.load(ctx, viewerID, username)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Unfollow(ctx, viewerID, target.ID); err != nil {
		return nil, err
	}
	target.Following = false
	return target, nil
}
