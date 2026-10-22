package articles

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
	List(ctx context.Context, viewerID int64, f Filter, offset, limit int) ([]Article, int, error)
	GetBySlug(ctx context.Context, viewerID int64, slug string) (*Article, error)
	SlugTaken(ctx context.Context, slug string, excludeID int64) (bool, error)
	Create(ctx context.Context, n NewRecord) (int64, error)
	Update(ctx context.Context, id int64, u UpdateRecord) error
	Delete(ctx context.Context, id int64) error
	Favorite(ctx context.Context, userID, articleID int64, now time.Time) error
	Unfavorite(ctx context.Context, userID, articleID int64) error
	PopularTags(ctx context.Context) ([]string, error)
}

// ListParams are the validated query parameters of GET /articles.
type ListParams struct {
	Tag, Author, Favorited string
	Offset, Limit          int
}

// Service holds the article business rules.
type Service struct {
	repo repository
	v    *validation.Validator
	now  func() time.Time
}

// NewService creates the service.
func NewService(repo repository, v *validation.Validator) *Service {
	return &Service{repo: repo, v: v, now: func() time.Time { return time.Now().UTC() }}
}

const slugRetries = 5

func (s *Service) load(ctx context.Context, viewerID int64, slug string) (*Article, error) {
	a, err := s.repo.GetBySlug(ctx, viewerID, slug)
	if errors.Is(err, database.ErrNotFound) {
		return nil, httpx.NotFound("article")
	}
	return a, err
}

// List returns articles matching the filters (unknown author/favorited -> empty).
func (s *Service) List(ctx context.Context, viewerID int64, p ListParams) ([]Article, int, error) {
	return s.repo.List(ctx, viewerID,
		Filter{Tag: p.Tag, Author: p.Author, FavoritedBy: p.Favorited}, p.Offset, p.Limit)
}

// Feed returns articles written by authors the caller follows.
func (s *Service) Feed(ctx context.Context, userID int64, offset, limit int) ([]Article, int, error) {
	return s.repo.List(ctx, userID, Filter{FeedFor: userID}, offset, limit)
}

// Get returns one article.
func (s *Service) Get(ctx context.Context, viewerID int64, slug string) (*Article, error) {
	return s.load(ctx, viewerID, slug)
}

// uniqueSlug returns the first free slug for title: base, base-2, base-3, ...
// "feed" is reserved. excludeID lets an article keep its own slug on update.
func (s *Service) uniqueSlug(ctx context.Context, title string, excludeID int64) (string, error) {
	base := Slugify(title)
	for n := 1; ; n++ {
		cand := slugCandidate(base, n)
		if cand == reservedSlug {
			continue
		}
		taken, err := s.repo.SlugTaken(ctx, cand, excludeID)
		if err != nil {
			return "", err
		}
		if !taken {
			return cand, nil
		}
	}
}

// Create validates and stores a new article.
func (s *Service) Create(ctx context.Context, authorID int64, in NewArticleInput) (*Article, error) {
	title := strings.TrimSpace(in.Title)
	description := strings.TrimSpace(in.Description)
	body := strings.TrimSpace(in.Body)

	errs := validation.Errors{}
	s.v.Var(errs, "title", title, "required,max=100")
	s.v.Var(errs, "description", description, "required,max=200")
	s.v.Var(errs, "body", body, "required,max=20000")
	tags, msg := normalizeTags(in.TagList)
	if msg != "" {
		errs.Add("tagList", msg)
	}
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}

	for attempt := 0; attempt < slugRetries; attempt++ {
		slug, err := s.uniqueSlug(ctx, title, 0)
		if err != nil {
			return nil, err
		}
		_, err = s.repo.Create(ctx, NewRecord{
			AuthorID: authorID, Slug: slug, Title: title, Description: description,
			Body: body, Tags: tags, Now: s.now(),
		})
		if errors.Is(err, ErrDuplicateSlug) {
			continue // lost a race for the slug; recompute
		}
		if err != nil {
			return nil, err
		}
		return s.repo.GetBySlug(ctx, authorID, slug)
	}
	return nil, errors.New("could not allocate a unique slug")
}

// Update applies a partial update. Order: 404 -> 403 -> 422.
func (s *Service) Update(ctx context.Context, userID int64, slug string, in UpdateArticleInput) (*Article, error) {
	a, err := s.load(ctx, userID, slug)
	if err != nil {
		return nil, err
	}
	if a.AuthorID != userID {
		return nil, httpx.Forbidden("article")
	}

	errs := validation.Errors{}
	rec := UpdateRecord{}
	text := func(field string, o httpx.Optional[string], tag string) *string {
		if !o.Set {
			return nil
		}
		if o.Null {
			errs.Add(field, "can't be blank")
			return nil
		}
		v := strings.TrimSpace(o.Value)
		s.v.Var(errs, field, v, tag)
		if errs.Has(field) {
			return nil
		}
		return &v
	}
	rec.Title = text("title", in.Title, "required,max=100")
	rec.Description = text("description", in.Description, "required,max=200")
	rec.Body = text("body", in.Body, "required,max=20000")
	if in.TagList.Set {
		raw := in.TagList.Value // null -> nil -> clears all tags
		tags, msg := normalizeTags(raw)
		if msg != "" {
			errs.Add("tagList", msg)
		} else {
			rec.Tags = &tags
		}
	}
	if len(errs) > 0 {
		return nil, httpx.Validation(errs)
	}

	if rec.Title == nil && rec.Description == nil && rec.Body == nil && rec.Tags == nil {
		return a, nil // nothing to change
	}

	for attempt := 0; attempt < slugRetries; attempt++ {
		finalSlug := a.Slug
		rec.Slug = nil
		if rec.Title != nil && *rec.Title != a.Title {
			ns, err := s.uniqueSlug(ctx, *rec.Title, a.ID)
			if err != nil {
				return nil, err
			}
			rec.Slug = &ns
			finalSlug = ns
		}
		rec.Now = s.now()
		err := s.repo.Update(ctx, a.ID, rec)
		if errors.Is(err, ErrDuplicateSlug) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return s.repo.GetBySlug(ctx, userID, finalSlug)
	}
	return nil, errors.New("could not allocate a unique slug")
}

// Delete removes an article (author only). Order: 404 -> 403.
func (s *Service) Delete(ctx context.Context, userID int64, slug string) error {
	a, err := s.load(ctx, userID, slug)
	if err != nil {
		return err
	}
	if a.AuthorID != userID {
		return httpx.Forbidden("article")
	}
	return s.repo.Delete(ctx, a.ID)
}

// Favorite is idempotent and returns the updated article.
func (s *Service) Favorite(ctx context.Context, userID int64, slug string) (*Article, error) {
	a, err := s.load(ctx, userID, slug)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Favorite(ctx, userID, a.ID, s.now()); err != nil {
		return nil, err
	}
	return s.repo.GetBySlug(ctx, userID, a.Slug)
}

// Unfavorite is idempotent and returns the updated article.
func (s *Service) Unfavorite(ctx context.Context, userID int64, slug string) (*Article, error) {
	a, err := s.load(ctx, userID, slug)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Unfavorite(ctx, userID, a.ID); err != nil {
		return nil, err
	}
	return s.repo.GetBySlug(ctx, userID, a.Slug)
}

// Tags returns the 20 most used tags.
func (s *Service) Tags(ctx context.Context) ([]string, error) {
	return s.repo.PopularTags(ctx)
}
