package profiles

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/auth"
	"conduit/backend/internal/platform/httpx"
)

// Handler exposes the profile endpoints.
type Handler struct{ svc *Service }

// NewHandler creates the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Get handles GET /profiles/{username} (auth optional).
func (h *Handler) Get(c *echo.Context) error {
	p, err := h.svc.Get(c.Request().Context(), auth.UserID(c), c.Param("username"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ProfileResponse{Profile: ToDTO(*p)})
}

// Follow handles POST /profiles/{username}/follow.
func (h *Handler) Follow(c *echo.Context) error {
	p, err := h.svc.Follow(c.Request().Context(), auth.UserID(c), c.Param("username"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ProfileResponse{Profile: ToDTO(*p)})
}

// Unfollow handles DELETE /profiles/{username}/follow.
func (h *Handler) Unfollow(c *echo.Context) error {
	p, err := h.svc.Unfollow(c.Request().Context(), auth.UserID(c), c.Param("username"))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, ProfileResponse{Profile: ToDTO(*p)})
}
