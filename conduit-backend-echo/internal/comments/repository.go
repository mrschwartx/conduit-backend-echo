package comments

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"conduit/backend/internal/platform/database"
)

// Repository contains only SQL for comments.
type Repository struct{ db *sql.DB }

// NewRepository creates the repository.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const commentSelect = `
SELECT c.id, c.article_id, c.author_id, c.body, c.created_at, c.updated_at,
       u.username, u.bio, u.image,
       EXISTS(SELECT 1 FROM follows fo WHERE fo.follower_id = ? AND fo.followee_id = c.author_id)
FROM comments c
JOIN users u ON u.id = c.author_id`

func scanComment(sc interface{ Scan(dest ...any) error }) (Comment, error) {
	var c Comment
	err := sc.Scan(&c.ID, &c.ArticleID, &c.AuthorID, &c.Body, &c.CreatedAt, &c.UpdatedAt,
		&c.Author.Username, &c.Author.Bio, &c.Author.Image, &c.Author.Following)
	c.Author.ID = c.AuthorID
	return c, err
}

// ArticleIDBySlug resolves a slug to the article id.
func (r *Repository) ArticleIDBySlug(ctx context.Context, slug string) (int64, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM articles WHERE slug = ?`, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, database.ErrNotFound
	}
	return id, err
}

// Create inserts a comment and returns its id.
func (r *Repository) Create(ctx context.Context, articleID, authorID int64, body string, now time.Time) (int64, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO comments (article_id, author_id, body, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		articleID, authorID, body, now.UTC(), now.UTC())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Get loads one comment; Following is relative to viewerID.
func (r *Repository) Get(ctx context.Context, viewerID, id int64) (*Comment, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	c, err := scanComment(r.db.QueryRowContext(ctx, commentSelect+` WHERE c.id = ?`, viewerID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, database.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// ListByArticle returns the comments of an article, newest first, in one query.
func (r *Repository) ListByArticle(ctx context.Context, viewerID, articleID int64) ([]Comment, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx,
		commentSelect+` WHERE c.article_id = ? ORDER BY c.created_at DESC, c.id DESC`, viewerID, articleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		c, err := scanComment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Delete removes a comment.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx, `DELETE FROM comments WHERE id = ?`, id)
	return err
}
