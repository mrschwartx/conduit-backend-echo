package rest

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/config"
)

type AuthCookie struct {
	auth         config.Auth
	isProduction bool
	now          func() time.Time
}

func NewAuthCookie(auth config.Auth, isProduction bool) *AuthCookie {
	return &AuthCookie{auth: auth, isProduction: isProduction, now: time.Now}
}

func (a *AuthCookie) Set(c *echo.Context, token string) {
	cookie := a.base()
	cookie.Value = token
	cookie.MaxAge = int(a.auth.CookieMaxAge / time.Second)
	cookie.Expires = a.now().Add(a.auth.CookieMaxAge)
	c.SetCookie(cookie)
}

func (a *AuthCookie) Clear(c *echo.Context) {
	cookie := a.base()
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(0, 0)
	c.SetCookie(cookie)
}

func (a *AuthCookie) base() *http.Cookie {
	cookie := &http.Cookie{
		Name:     a.auth.CookieName,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if a.isProduction {
		cookie.SameSite = http.SameSiteNoneMode
		cookie.Secure = true
		cookie.Domain = a.auth.CookieDomain
	}
	return cookie
}
