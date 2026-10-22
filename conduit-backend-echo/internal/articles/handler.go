package articles

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/auth"
	"conduit/backend/internal/platform/httpx"
)

// Handler exposes the article endpoints.
type Handler struct{ svc *Service }

// NewHandler creates the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List handles GET /articles (auth optional).
func (h *Handler) List(c *echo.Context) error {
	offset, limit, errs := httpx.ParsePagination(c.QueryParam("offset"), c.QueryParam("limit"))
	if len(errs) > 0 {
		return httpx.Validation(errs)
	}
	items, total, err := h.svc.List(c.Request().Context(), auth.UserID(c), ListParams{
		Tag:       c.QueryParam("tag"),
		Author:    c.QueryParam("author"),
		Favorited: c.QueryParam("favorited"),
		Offset:    offset,
		Limit:     limit,
	})
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, toListResponse(items, total))
}

// Feed handles GET /articles/feed (auth required).
func (h *Handler) Feed(c *echo.Context) error {
	offset, limit, errs := httpx.ParsePagination(c.QueryParam("offset"), c.QueryParam("limit"))
	if len(errs) > 0 {
		return httpx.Validation(errs)
	}
	items, total, err := h.svc.Feed(c.Request().Context(), auth.UserID(c), offset, limit)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, toListResponse(items, total))
}

// Get handles GET /articles/{slug} (auth optional).
func (h *Handler) Get(c *echo.Context) error {
	a, err := h.svc.Get(c.Request().Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ArticleResponse{Article: toDTO(a)})
}

// Create handles POST /articles.
func (h *Handler) Create(c *echo.Context) error {
	var req CreateRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.Article == nil {
		return httpx.ValidationField("article", "can't be blank")
	}
	a, err := h.svc.Create(c.Request().Context(), auth.UserID(c), *req.Article)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusCreated, ArticleResponse{Article: toDTO(a)})
}

// Update handles PUT /articles/{slug}.
func (h *Handler) Update(c *echo.Context) error {
	var req UpdateRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.Article == nil {
		return httpx.ValidationField("article", "can't be blank")
	}
	a, err := h.svc.Update(c.Request().Context(), auth.UserID(c), c.Param("slug"), *req.Article)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ArticleResponse{Article: toDTO(a)})
}

// Delete handles DELETE /articles/{slug}.
func (h *Handler) Delete(c *echo.Context) error {
	if err := h.svc.Delete(c.Request().Context(), auth.UserID(c), c.Param("slug")); err != nil {
		return err
	}
	return httpx.NoContent(c)
}

// Favorite handles POST /articles/{slug}/favorite.
func (h *Handler) Favorite(c *echo.Context) error {
	a, err := h.svc.Favorite(c.Request().Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ArticleResponse{Article: toDTO(a)})
}

// Unfavorite handles DELETE /articles/{slug}/favorite.
func (h *Handler) Unfavorite(c *echo.Context) error {
	a, err := h.svc.Unfavorite(c.Request().Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ArticleResponse{Article: toDTO(a)})
}

// Tags handles GET /tags.
func (h *Handler) Tags(c *echo.Context) error {
	tags, err := h.svc.Tags(c.Request().Context())
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, TagsResponse{Tags: tags})
}
