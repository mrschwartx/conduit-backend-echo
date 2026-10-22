package articles

import (
	"conduit/backend/internal/platform/httpx"
	"conduit/backend/internal/profiles"
)

// NewArticleInput is the "article" object of POST /articles.
type NewArticleInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Body        string   `json:"body"`
	TagList     []string `json:"tagList"`
}

// CreateRequest is the POST /articles envelope.
type CreateRequest struct {
	Article *NewArticleInput `json:"article"`
}

// UpdateArticleInput is the "article" object of PUT /articles/{slug}.
type UpdateArticleInput struct {
	Title       httpx.Optional[string]   `json:"title"`
	Description httpx.Optional[string]   `json:"description"`
	Body        httpx.Optional[string]   `json:"body"`
	TagList     httpx.Optional[[]string] `json:"tagList"`
}

// UpdateRequest is the PUT /articles/{slug} envelope.
type UpdateRequest struct {
	Article *UpdateArticleInput `json:"article"`
}

// ArticleDTO matches the contract "Article" schema.
type ArticleDTO struct {
	Slug           string              `json:"slug"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	Body           string              `json:"body"`
	TagList        []string            `json:"tagList"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
	Favorited      bool                `json:"favorited"`
	FavoritesCount int                 `json:"favoritesCount"`
	Author         profiles.ProfileDTO `json:"author"`
}

// ArticleListItemDTO is the list/feed item: same as ArticleDTO WITHOUT body.
type ArticleListItemDTO struct {
	Slug           string              `json:"slug"`
	Title          string              `json:"title"`
	Description    string              `json:"description"`
	TagList        []string            `json:"tagList"`
	CreatedAt      string              `json:"createdAt"`
	UpdatedAt      string              `json:"updatedAt"`
	Favorited      bool                `json:"favorited"`
	FavoritesCount int                 `json:"favoritesCount"`
	Author         profiles.ProfileDTO `json:"author"`
}

// ArticleResponse is the {"article": ...} envelope.
type ArticleResponse struct {
	Article ArticleDTO `json:"article"`
}

// ArticlesResponse is the {"articles": [...], "articlesCount": n} envelope.
type ArticlesResponse struct {
	Articles      []ArticleListItemDTO `json:"articles"`
	ArticlesCount int                  `json:"articlesCount"`
}

// TagsResponse is the {"tags": [...]} envelope.
type TagsResponse struct {
	Tags []string `json:"tags"`
}

func tagsOrEmpty(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

func toDTO(a *Article) ArticleDTO {
	return ArticleDTO{
		Slug: a.Slug, Title: a.Title, Description: a.Description, Body: a.Body,
		TagList:   tagsOrEmpty(a.TagList),
		CreatedAt: httpx.FormatTime(a.CreatedAt), UpdatedAt: httpx.FormatTime(a.UpdatedAt),
		Favorited: a.Favorited, FavoritesCount: a.FavoritesCount,
		Author: profiles.ToDTO(a.Author),
	}
}

func toListResponse(items []Article, total int) ArticlesResponse {
	out := make([]ArticleListItemDTO, 0, len(items))
	for i := range items {
		a := &items[i]
		out = append(out, ArticleListItemDTO{
			Slug: a.Slug, Title: a.Title, Description: a.Description,
			TagList:   tagsOrEmpty(a.TagList),
			CreatedAt: httpx.FormatTime(a.CreatedAt), UpdatedAt: httpx.FormatTime(a.UpdatedAt),
			Favorited: a.Favorited, FavoritesCount: a.FavoritesCount,
			Author: profiles.ToDTO(a.Author),
		})
	}
	return ArticlesResponse{Articles: out, ArticlesCount: total}
}
