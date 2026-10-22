// Package server wires repositories, services, handlers and middleware into a
// single http.Handler. It is used by cmd/api and by the integration tests.
package server

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/articles"
	"conduit/backend/internal/comments"
	"conduit/backend/internal/identity"
	"conduit/backend/internal/platform/auth"
	"conduit/backend/internal/platform/httpx"
	"conduit/backend/internal/platform/validation"
	"conduit/backend/internal/profiles"
)

// MaxBodyBytes is the request body limit (1 MB).
const MaxBodyBytes = 1 << 20

// Deps are the external dependencies of the HTTP server.
type Deps struct {
	DB          *sql.DB
	Logger      *slog.Logger
	JWTSecret   string
	JWTTTL      time.Duration
	BcryptCost  int
	CORSOrigins []string
}

// New builds the full HTTP handler (Echo router wrapped in net/http middleware).
func New(d Deps) (http.Handler, error) {
	v := validation.New()
	tokens := auth.NewService(d.JWTSecret, d.JWTTTL)

	identitySvc, err := identity.NewService(identity.NewRepository(d.DB), tokens, v, d.BcryptCost)
	if err != nil {
		return nil, err
	}
	identityH := identity.NewHandler(identitySvc)
	profilesH := profiles.NewHandler(profiles.NewService(profiles.NewRepository(d.DB)))
	articlesH := articles.NewHandler(articles.NewService(articles.NewRepository(d.DB), v))
	commentsH := comments.NewHandler(comments.NewService(comments.NewRepository(d.DB), v))

	e := echo.New()
	e.HTTPErrorHandler = httpx.ErrorHandler()

	e.GET("/health", func(c *echo.Context) error {
		w := c.Response()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		return err
	})

	required := auth.Required(tokens)
	optional := auth.Optional(tokens)

	g := e.Group("/api/v1")

	// User and authentication
	g.POST("/users/login", identityH.Login)
	g.POST("/users", identityH.Register)
	g.GET("/user", identityH.Current, required)
	g.PUT("/user", identityH.Update, required)

	// Profiles
	g.GET("/profiles/:username", profilesH.Get, optional)
	g.POST("/profiles/:username/follow", profilesH.Follow, required)
	g.DELETE("/profiles/:username/follow", profilesH.Unfollow, required)

	// Articles. "/articles/feed" is a static route and wins over "/articles/:slug".
	g.GET("/articles/feed", articlesH.Feed, required)
	g.GET("/articles", articlesH.List, optional)
	g.POST("/articles", articlesH.Create, required)
	g.GET("/articles/:slug", articlesH.Get, optional)
	g.PUT("/articles/:slug", articlesH.Update, required)
	g.DELETE("/articles/:slug", articlesH.Delete, required)

	// Favorites
	g.POST("/articles/:slug/favorite", articlesH.Favorite, required)
	g.DELETE("/articles/:slug/favorite", articlesH.Unfavorite, required)

	// Comments
	g.GET("/articles/:slug/comments", commentsH.List, optional)
	g.POST("/articles/:slug/comments", commentsH.Create, required)
	g.DELETE("/articles/:slug/comments/:id", commentsH.Delete, required)

	// Tags
	g.GET("/tags", articlesH.Tags)

	return httpx.Chain(e,
		httpx.RequestID(d.Logger),
		httpx.AccessLog(),
		httpx.Recover(),
		httpx.SecureHeaders(),
		httpx.CORS(d.CORSOrigins),
		httpx.BodyLimit(MaxBodyBytes),
	), nil
}
