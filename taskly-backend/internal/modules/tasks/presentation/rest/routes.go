package rest

import "github.com/labstack/echo/v5"

func RegisterRoutes(e *echo.Echo, handler *Handler, authenticate echo.MiddlewareFunc) {
	group := e.Group("/api/v1/tasks", authenticate)
	group.GET("/user/:id", handler.ListByUser)
	group.GET("/:id", handler.Get)
	group.POST("/create", handler.Create)
	group.PATCH("/:id", handler.Update)
	group.DELETE("/:id", handler.Delete)
}
