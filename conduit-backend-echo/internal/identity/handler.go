package identity

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/platform/auth"
	"conduit/backend/internal/platform/httpx"
)

// Handler exposes the identity endpoints.
type Handler struct{ svc *Service }

// NewHandler creates the handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// Register handles POST /users.
func (h *Handler) Register(c *echo.Context) error {
	var req RegisterRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.User == nil {
		return httpx.ValidationField("user", "can't be blank")
	}
	res, err := h.svc.Register(c.Request().Context(), *req.User)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusCreated, toResponse(res))
}

// Login handles POST /users/login.
func (h *Handler) Login(c *echo.Context) error {
	var req LoginRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.User == nil {
		return httpx.ValidationField("user", "can't be blank")
	}
	res, err := h.svc.Login(c.Request().Context(), *req.User)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, toResponse(res))
}

// Current handles GET /user.
func (h *Handler) Current(c *echo.Context) error {
	res, err := h.svc.Current(c.Request().Context(), auth.UserID(c))
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, toResponse(res))
}

// Update handles PUT /user.
func (h *Handler) Update(c *echo.Context) error {
	var req UpdateUserRequest
	if err := httpx.Bind(c, &req); err != nil {
		return err
	}
	if req.User == nil {
		return httpx.ValidationField("user", "can't be blank")
	}
	res, err := h.svc.Update(c.Request().Context(), auth.UserID(c), *req.User)
	if err != nil {
		return err
	}
	return httpx.JSON(c, http.StatusOK, toResponse(res))
}
