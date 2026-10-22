package comments

import (
	"context"
	"errors"
	"strings"
	"time"

	"conduit/backend/internal/platform/database"
	"conduit/backend/internal/platform/httpx"
	"conduit/backend/internal/platform/validation"
)

type repository interface {
	ArticleIDBySlug(ctx context.Context, slug string) (int64, error)
	Create(ctx context.Context, articleID, authorID int64, body string, now time.Time) (int64, error)
	Get(ctx context.Context, viewerID, id int64) (*Comment, error)
	ListByArticle(ctx context.Context, viewerID, articleID int64) ([]Comment, error)
	Delete(ctx context.Context, id int64) error
}

// Service holds the comment business rules.
type Service struct {
	repo repository
	v    *validation.Validator
	now  func() time.Time
}

// NewService creates the service.
func NewService(repo repository, v *validation.Validator) *Service {
	return &Service{repo: repo, v: v, now: func() time.Time { return time.Now().UTC() }}
}

func (s *Service) article(ctx context.Context, slug string) (int64, error) {
	id, err := s.repo.ArticleIDBySlug(ctx, slug)
	if errors.Is(err, database.ErrNotFound) {
		return 0, httpx.NotFound("article")
	}
	return id, err
}

// List returns the comments of an article, newest first.
func (s *Service) List(ctx context.Context, viewerID int64, slug string) ([]Comment, error) {
	articleID, err := s.article(ctx, slug)
	if err != nil {
		return nil, err
	}
	return s.repo.ListByArticle(ctx, viewerID, articleID)
}

// Create adds a comment. Order: 404 (article) -> 422.
func (s *Service) Create(ctx context.Context, userID int64, slug string, in NewCommentInput) (*Comment, error) {
	articleID, err := s.article(ctx, slug)
	if err != nil {
		return nil, err
	}
	body := strings.TrimSpace(in.Body)
	errs := validation.Errors{}
	s.v.Var(errs, "body", body, "required,max=1000")
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}
	id, err := s.repo.Create(ctx, articleID, userID, body, s.now())
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, userID, id)
}

// Delete removes a comment (author only). A comment that does not belong to the
// given slug is a 404. Order: 404 article -> 404 comment -> 403.
func (s *Service) Delete(ctx context.Context, userID int64, slug string, commentID int64) error {
	articleID, err := s.article(ctx, slug)
	if err != nil {
		return err
	}
	c, err := s.repo.Get(ctx, userID, commentID)
	if errors.Is(err, database.ErrNotFound) || (err == nil && c.ArticleID != articleID) {
		return httpx.NotFound("comment")
	}
	if err != nil {
		return err
	}
	if c.AuthorID != userID {
		return httpx.Forbidden("comment")
	}
	return s.repo.Delete(ctx, c.ID)
}
