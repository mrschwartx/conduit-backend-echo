package bootstrap

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"taskly/backend/internal/shared/infrastructure/httpx"
)

type welcomeResponse struct {
	Message string `json:"message"`
}

func NewApp(container *Container) (*echo.Echo, error) {
	cfg := container.Config
	logger := container.Logger

	e := echo.New()
	e.Logger = logger
	e.HTTPErrorHandler = httpx.NewErrorHandler(logger)

	cors, err := middleware.CORSConfig{
		AllowOrigins:     []string{strings.TrimRight(cfg.ClientURL, "/")},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderContentType, echo.HeaderAuthorization},
		AllowCredentials: true,
	}.ToMiddleware()
	if err != nil {
		return nil, err
	}
	e.Use(cors)

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		HandleError: true,
		LogMethod:   true,
		LogURIPath:  true,
		LogStatus:   true,
		LogLatency:  true,
		LogValuesFunc: func(_ *echo.Context, values middleware.RequestLoggerValues) error {
			logger.Info("HTTP request",
				"method", values.Method,
				"path", values.URIPath,
				"status", values.Status,
				"durationMs", float64(values.Latency.Microseconds())/1000,
			)
			return nil
		},
	}))

	e.StaticFS("/", os.DirFS(cfg.Storage.PublicDir), noSniff)
	e.GET("/", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, welcomeResponse{Message: "Welcome to the Taskly API"})
	})

	container.Users.RegisterRoutes(e)
	container.Tasks.RegisterRoutes(e)
	return e, nil
}

func noSniff(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		c.Response().Header().Set("X-Content-Type-Options", "nosniff")
		return next(c)
	}
}
