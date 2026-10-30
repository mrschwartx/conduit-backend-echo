package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/config"
	"taskly/backend/internal/modules/users/application"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/httpx"
)

type stubTokens struct{}

func (stubTokens) Issue(string) (string, error) { return "", nil }

func (stubTokens) Verify(token string) (string, error) {
	if token == "valid" {
		return "user-1", nil
	}
	return "", http.ErrNoCookie
}

func authConfig() config.Auth {
	return config.Auth{CookieName: "taskly_token", CookieMaxAge: time.Hour, CookieDomain: ".example.com"}
}

func newContext() (*echo.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	return echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), recorder), recorder
}

func TestAuthCookieDevelopment(t *testing.T) {
	c, recorder := newContext()
	cookie := NewAuthCookie(authConfig(), false)
	cookie.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }

	cookie.Set(c, "abc")
	header := recorder.Header().Get("Set-Cookie")

	for _, expected := range []string{"taskly_token=abc", "Path=/", "HttpOnly", "SameSite=Lax", "Max-Age=3600"} {
		if !strings.Contains(header, expected) {
			t.Errorf("%q lacks %q", header, expected)
		}
	}
	for _, unexpected := range []string{"Secure", "Domain="} {
		if strings.Contains(header, unexpected) {
			t.Errorf("%q must not contain %q", header, unexpected)
		}
	}
}

func TestAuthCookieProduction(t *testing.T) {
	c, recorder := newContext()
	NewAuthCookie(authConfig(), true).Set(c, "abc")
	header := recorder.Header().Get("Set-Cookie")

	for _, expected := range []string{"Secure", "SameSite=None", "Domain=example.com", "HttpOnly"} {
		if !strings.Contains(header, expected) {
			t.Errorf("%q lacks %q", header, expected)
		}
	}
}

func TestAuthCookieClear(t *testing.T) {
	c, recorder := newContext()
	NewAuthCookie(authConfig(), false).Clear(c)
	header := recorder.Header().Get("Set-Cookie")
	if !strings.Contains(header, "Max-Age=0") {
		t.Fatalf("unexpected clear header %q", header)
	}
}

func TestAuthenticateMiddleware(t *testing.T) {
	verify := application.NewVerifyAccessToken(stubTokens{})
	handler := Authenticate(verify, "taskly_token")(func(c *echo.Context) error {
		return c.String(http.StatusOK, httpx.AuthUserID(c))
	})

	cases := []struct {
		name    string
		cookie  string
		kind    sharedomain.ErrorKind
		message string
		userID  string
	}{
		{"valid", "valid", "", "", "user-1"},
		{"missing", "", sharedomain.KindAuthentication, "Unauthorized: Token not provided", ""},
		{"invalid", "bad", sharedomain.KindForbidden, "Forbidden: Invalid token", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.cookie != "" {
				request.AddCookie(&http.Cookie{Name: "taskly_token", Value: tc.cookie})
			}
			recorder := httptest.NewRecorder()
			err := handler(echo.New().NewContext(request, recorder))

			if tc.kind == "" {
				if err != nil || recorder.Body.String() != tc.userID {
					t.Fatalf("unexpected result: %v %q", err, recorder.Body.String())
				}
				return
			}
			if !sharedomain.IsKind(err, tc.kind) || err.Error() != tc.message {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestEnsureOwnAccountMiddleware(t *testing.T) {
	handler := EnsureOwnAccount("nope")(func(c *echo.Context) error { return c.NoContent(http.StatusNoContent) })

	run := func(requester, id string) error {
		c, _ := newContext()
		c.SetPath("/users/:id")
		c.SetPathValues(echo.PathValues{{Name: "id", Value: id}})
		httpx.SetAuthUser(c, requester)
		return handler(c)
	}

	if err := run("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA"); err != nil {
		t.Fatalf("own account must pass: %v", err)
	}
	err := run("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")
	if !sharedomain.IsKind(err, sharedomain.KindAuthentication) || err.Error() != "nope" {
		t.Fatalf("foreign account must be rejected: %v", err)
	}
}

func TestDetectExtension(t *testing.T) {
	valid := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "IMAGE/WEBP": ".webp", "image/gif; charset=x": ".gif"}
	for input, want := range valid {
		if got, ok := detectExtension(input); !ok || got != want {
			t.Errorf("detectExtension(%q) = %q %v", input, got, ok)
		}
	}
	for _, input := range []string{"", "text/html", "image/svg+xml", "application/octet-stream", "%%%"} {
		if _, ok := detectExtension(input); ok {
			t.Errorf("detectExtension(%q) must fail", input)
		}
	}
}

func TestNewAvatarFilename(t *testing.T) {
	first, err := newAvatarFilename(".png")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first, "avatar-") || !strings.HasSuffix(first, ".png") {
		t.Fatalf("unexpected filename %q", first)
	}
}
