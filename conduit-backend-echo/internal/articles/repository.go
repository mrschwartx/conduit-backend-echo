package articles

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"conduit/backend/internal/platform/database"
)

// ErrDuplicateSlug is returned when the unique slug index is violated (race).
var ErrDuplicateSlug = errors.New("duplicate slug")

// Repository contains only SQL for articles, tags and favorites.
type Repository struct{ db *sql.DB }

// NewRepository creates the repository.
func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

// articleSelect assembles author profile, favoritesCount, favorited and following
// in ONE query (no per-row queries). The two leading "?" are the viewer id.
const articleSelectHead = `
SELECT a.id, a.author_id, a.slug, a.title, a.description, `

const articleSelectTail = `, a.created_at, a.updated_at,
       (SELECT COUNT(*) FROM favorites f WHERE f.article_id = a.id),
       EXISTS(SELECT 1 FROM favorites f2 WHERE f2.article_id = a.id AND f2.user_id = ?),
       u.username, u.bio, u.image,
       EXISTS(SELECT 1 FROM follows fo WHERE fo.follower_id = ? AND fo.followee_id = a.author_id)
FROM articles a
JOIN users u ON u.id = a.author_id`

const articleFrom = `FROM articles a JOIN users u ON u.id = a.author_id`

func (f Filter) where() (string, []any) {
	var conds []string
	var args []any
	if f.Slug != "" {
		conds = append(conds, "a.slug = ?")
		args = append(args, f.Slug)
	}
	if f.Tag != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM article_tags atg JOIN tags t ON t.id = atg.tag_id
			WHERE atg.article_id = a.id AND t.name = ?)`)
		args = append(args, f.Tag)
	}
	if f.Author != "" {
		conds = append(conds, "u.username = ?")
		args = append(args, f.Author)
	}
	if f.FavoritedBy != "" {
		conds = append(conds, `EXISTS (SELECT 1 FROM favorites ff JOIN users fu ON fu.id = ff.user_id
			WHERE ff.article_id = a.id AND fu.username = ?)`)
		args = append(args, f.FavoritedBy)
	}
	if f.FeedFor != 0 {
		conds = append(conds, "a.author_id IN (SELECT fe.followee_id FROM follows fe WHERE fe.follower_id = ?)")
		args = append(args, f.FeedFor)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func (r *Repository) query(ctx context.Context, viewerID int64, f Filter, offset, limit int, withBody bool) ([]Article, int, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()

	whereSQL, whereArgs := f.where()

	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) "+articleFrom+whereSQL, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []Article{}, 0, nil
	}

	bodyCol := "''"
	if withBody {
		bodyCol = "a.body"
	}
	q := articleSelectHead + bodyCol + articleSelectTail + whereSQL +
		" ORDER BY a.created_at DESC, a.id DESC LIMIT ? OFFSET ?"
	args := append([]any{viewerID, viewerID}, whereArgs...)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]Article, 0, limit)
	index := make(map[int64]int)
	for rows.Next() {
		var a Article
		if err := rows.Scan(&a.ID, &a.AuthorID, &a.Slug, &a.Title, &a.Description, &a.Body,
			&a.CreatedAt, &a.UpdatedAt, &a.FavoritesCount, &a.Favorited,
			&a.Author.Username, &a.Author.Bio, &a.Author.Image, &a.Author.Following); err != nil {
			return nil, 0, err
		}
		a.Author.ID = a.AuthorID
		a.TagList = []string{}
		index[a.ID] = len(items)
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()

	if err := r.loadTags(ctx, items, index); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// loadTags fetches the tags of all listed articles with a single IN query.
func (r *Repository) loadTags(ctx context.Context, items []Article, index map[int64]int) error {
	if len(items) == 0 {
		return nil
	}
	args := make([]any, 0, len(items))
	for i := range items {
		args = append(args, items[i].ID)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT atg.article_id, t.name
		   FROM article_tags atg JOIN tags t ON t.id = atg.tag_id
		  WHERE atg.article_id IN (`+placeholders(len(items))+`)
		  ORDER BY atg.article_id, atg.sort_order`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var articleID int64
		var name string
		if err := rows.Scan(&articleID, &name); err != nil {
			return err
		}
		i := index[articleID]
		items[i].TagList = append(items[i].TagList, name)
	}
	return rows.Err()
}

// List returns a page of articles (without body) and the total match count.
func (r *Repository) List(ctx context.Context, viewerID int64, f Filter, offset, limit int) ([]Article, int, error) {
	return r.query(ctx, viewerID, f, offset, limit, false)
}

// GetBySlug returns one article including its body.
func (r *Repository) GetBySlug(ctx context.Context, viewerID int64, slug string) (*Article, error) {
	items, _, err := r.query(ctx, viewerID, Filter{Slug: slug}, 0, 1, true)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, database.ErrNotFound
	}
	return &items[0], nil
}

// SlugTaken reports whether another article already uses slug.
func (r *Repository) SlugTaken(ctx context.Context, slug string, excludeID int64) (bool, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	var one int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM articles WHERE slug = ? AND id <> ? LIMIT 1`, slug, excludeID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func mapDuplicate(err error) error {
	if msg, ok := database.IsDuplicate(err); ok && strings.Contains(msg, "uq_articles_slug") {
		return ErrDuplicateSlug
	}
	return err
}

// Create inserts the article and its tags in one transaction.
func (r *Repository) Create(ctx context.Context, n NewRecord) (int64, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	res, err := tx.ExecContext(ctx,
		`INSERT INTO articles (author_id, slug, title, description, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		n.AuthorID, n.Slug, n.Title, n.Description, n.Body, n.Now.UTC(), n.Now.UTC())
	if err != nil {
		return 0, mapDuplicate(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := setTags(ctx, tx, id, n.Tags); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// Update applies the changed fields (and replaces tags when provided) atomically.
func (r *Repository) Update(ctx context.Context, id int64, u UpdateRecord) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	sets := []string{"updated_at = ?"}
	args := []any{u.Now.UTC()}
	if u.Slug != nil {
		sets, args = append(sets, "slug = ?"), append(args, *u.Slug)
	}
	if u.Title != nil {
		sets, args = append(sets, "title = ?"), append(args, *u.Title)
	}
	if u.Description != nil {
		sets, args = append(sets, "description = ?"), append(args, *u.Description)
	}
	if u.Body != nil {
		sets, args = append(sets, "body = ?"), append(args, *u.Body)
	}
	args = append(args, id)
	if _, err := tx.ExecContext(ctx, "UPDATE articles SET "+strings.Join(sets, ", ")+" WHERE id = ?", args...); err != nil {
		return mapDuplicate(err)
	}
	if u.Tags != nil {
		if err := setTags(ctx, tx, id, *u.Tags); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// setTags replaces the tag set of an article, preserving the given order.
func setTags(ctx context.Context, tx *sql.Tx, articleID int64, tags []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM article_tags WHERE article_id = ?`, articleID); err != nil {
		return err
	}
	if len(tags) == 0 {
		return nil
	}
	names := make([]any, len(tags))
	for i, t := range tags {
		names[i] = t
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO tags (name) VALUES `+strings.TrimSuffix(strings.Repeat("(?),", len(tags)), ","),
		names...); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx,
		`SELECT id, name FROM tags WHERE name IN (`+placeholders(len(tags))+`)`, names...)
	if err != nil {
		return err
	}
	ids := make(map[string]int64, len(tags))
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return err
		}
		ids[name] = id
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	vals := make([]string, 0, len(tags))
	args := make([]any, 0, len(tags)*3)
	for pos, t := range tags {
		vals = append(vals, "(?, ?, ?)")
		args = append(args, articleID, ids[t], pos)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO article_tags (article_id, tag_id, sort_order) VALUES `+strings.Join(vals, ", "), args...)
	return err
}

// Delete removes the article together with its comments, favorites and tag links.
func (r *Repository) Delete(ctx context.Context, id int64) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after commit

	for _, q := range []string{
		`DELETE FROM comments WHERE article_id = ?`,
		`DELETE FROM favorites WHERE article_id = ?`,
		`DELETE FROM article_tags WHERE article_id = ?`,
		`DELETE FROM articles WHERE id = ?`,
	} {
		if _, err := tx.ExecContext(ctx, q, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Favorite is idempotent.
func (r *Repository) Favorite(ctx context.Context, userID, articleID int64, now time.Time) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`INSERT IGNORE INTO favorites (user_id, article_id, created_at) VALUES (?, ?, ?)`,
		userID, articleID, now.UTC())
	return err
}

// Unfavorite is idempotent.
func (r *Repository) Unfavorite(ctx context.Context, userID, articleID int64) error {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM favorites WHERE user_id = ? AND article_id = ?`, userID, articleID)
	return err
}

// PopularTags returns the 20 most used tags (usage desc, then alphabetical).
func (r *Repository) PopularTags(ctx context.Context) ([]string, error) {
	ctx, cancel := database.WithTimeout(ctx)
	defer cancel()
	rows, err := r.db.QueryContext(ctx, `
		SELECT t.name
		  FROM article_tags atg
		  JOIN tags t ON t.id = atg.tag_id
		 GROUP BY t.id, t.name
		 ORDER BY COUNT(*) DESC, t.name ASC
		 LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}
