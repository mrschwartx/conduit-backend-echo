package rest

import (
	"github.com/labstack/echo/v5"

	"taskly/backend/internal/modules/users/application"
	"taskly/backend/internal/shared/infrastructure/httpx"
)

func Authenticate(verify *application.VerifyAccessToken, cookieName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			token := ""
			if cookie, err := c.Cookie(cookieName); err == nil {
				token = cookie.Value
			}
			userID, err := verify.Execute(token)
			if err != nil {
				return err
			}
			httpx.SetAuthUser(c, userID)
			return next(c)
		}
	}
}

func EnsureOwnAccount(message string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if err := application.AssertOwnAccount(httpx.AuthUserID(c), c.Param("id"), message); err != nil {
				return err
			}
			return next(c)
		}
	}
}
