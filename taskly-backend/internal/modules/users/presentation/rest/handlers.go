package rest

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/modules/users/application"
	"taskly/backend/internal/modules/users/domain"
	"taskly/backend/internal/shared/infrastructure/httpx"
)

type MessageResponse struct {
	Message string `json:"message"`
}

type AuthHandler struct {
	register     *application.RegisterUser
	authenticate *application.AuthenticateUser
	cookie       *AuthCookie
}

func NewAuthHandler(register *application.RegisterUser, authenticate *application.AuthenticateUser, cookie *AuthCookie) *AuthHandler {
	return &AuthHandler{register: register, authenticate: authenticate, cookie: cookie}
}

func (h *AuthHandler) SignUp(c *echo.Context) error {
	var input application.RegisterUserInput
	if err := httpx.DecodeJSON(c, &input); err != nil {
		return err
	}
	result, err := h.register.Execute(c.Request().Context(), input)
	if err != nil {
		return err
	}
	h.cookie.Set(c, result.Token)
	return c.JSON(http.StatusOK, result.User)
}

func (h *AuthHandler) SignIn(c *echo.Context) error {
	var input application.AuthenticateUserInput
	if err := httpx.DecodeJSON(c, &input); err != nil {
		return err
	}
	result, err := h.authenticate.Execute(c.Request().Context(), input)
	if err != nil {
		return err
	}
	h.cookie.Set(c, result.Token)
	return c.JSON(http.StatusOK, result.User)
}

func (h *AuthHandler) SignOut(c *echo.Context) error {
	h.cookie.Clear(c)
	return c.JSON(http.StatusOK, MessageResponse{Message: "Sign out successful"})
}

type UsersHandler struct {
	get      *application.GetUser
	list     *application.ListUsers
	update   *application.UpdateUser
	delete   *application.DeleteUser
	avatar   *application.ChangeUserAvatar
	uploader *AvatarUploader
	cookie   *AuthCookie
}

func NewUsersHandler(
	get *application.GetUser,
	list *application.ListUsers,
	update *application.UpdateUser,
	deleteUser *application.DeleteUser,
	avatar *application.ChangeUserAvatar,
	uploader *AvatarUploader,
	cookie *AuthCookie,
) *UsersHandler {
	return &UsersHandler{
		get: get, list: list, update: update, delete: deleteUser,
		avatar: avatar, uploader: uploader, cookie: cookie,
	}
}

func (h *UsersHandler) List(c *echo.Context) error {
	query := c.QueryParams()
	users, err := h.list.Execute(c.Request().Context(), application.ListUsersInput{
		Username: query["username"],
		Email:    query["email"],
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, users)
}

func (h *UsersHandler) Get(c *echo.Context) error {
	user, err := h.get.Execute(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

func (h *UsersHandler) Update(c *echo.Context) error {
	var changes application.UserChanges
	if err := httpx.DecodeJSON(c, &changes); err != nil {
		return err
	}
	user, err := h.update.Execute(c.Request().Context(), application.UpdateUserInput{
		RequesterID: httpx.AuthUserID(c),
		UserID:      c.Param("id"),
		Changes:     changes,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}

func (h *UsersHandler) Delete(c *echo.Context) error {
	err := h.delete.Execute(c.Request().Context(), application.DeleteUserInput{
		RequesterID: httpx.AuthUserID(c),
		UserID:      c.Param("id"),
	})
	if err != nil {
		return err
	}
	h.cookie.Clear(c)
	return c.JSON(http.StatusOK, MessageResponse{Message: "User has been deleted"})
}

func (h *UsersHandler) UploadAvatar(c *echo.Context) error {
	filename, err := h.uploader.Receive(c)
	if err != nil {
		return err
	}
	avatarPath := ""
	if filename != "" {
		avatarPath = domain.ToAvatarPath(filename)
	}
	user, err := h.avatar.Execute(c.Request().Context(), application.ChangeUserAvatarInput{
		RequesterID: httpx.AuthUserID(c),
		UserID:      c.Param("id"),
		AvatarPath:  avatarPath,
	})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, user)
}
