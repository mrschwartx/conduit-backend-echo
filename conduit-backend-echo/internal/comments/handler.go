package comments

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/auth"
	"conduit/backend/internal/platform/httpx"
)

// Handler exposes the comment endpoints.
type Handler struct{ svc *Service }

// NewHandler creates the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// List handles GET /articles/{slug}/comments (auth optional).
func (h *Handler) List(c *echo.Context) error {
	items, err := h.svc.List(c.Request().Context(), auth.UserID(c), c.Param("slug"))
	if err != nil {
		return err
	}
	out := make([]CommentDTO, 0, len(items))
	for i := range items {
		out = append(out, toDTO(&items[i]))
	}
	return httpx.JSON(c, http.StatusOK, CommentsResponse{Comments: out})
}

// Create handles POST /articles/{slug}/comments.
func (h *Handler) Create(c *echo.Context) error {
	var req CreateRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.Comment == nil {
		return httpx.ValidationField("comment", "can't be blank")
	}
	cm, err := h.svc.Create(c.Request().Context(), auth.UserID(c), c.Param("slug"), *req.Comment)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusCreated, CommentResponse{Comment: toDTO(cm)})
}

// Delete handles DELETE /articles/{slug}/comments/{id}.
func (h *Handler) Delete(c *echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		return httpx.ValidationField("id", "is invalid")
	}
	if err := h.svc.Delete(c.Request().Context(), auth.UserID(c), c.Param("slug"), id); err != nil {
		return err
	}
	return httpx.NoContent(c)
}
