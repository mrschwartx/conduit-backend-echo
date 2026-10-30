package rest

import "github.com/labstack/echo/v5"

func RegisterAuthRoutes(e *echo.Echo, handler *AuthHandler) {
	group := e.Group("/api/v1/auth")
	group.POST("/signup", handler.SignUp)
	group.POST("/signin", handler.SignIn)
	group.GET("/signout", handler.SignOut)
}

func RegisterUsersRoutes(e *echo.Echo, handler *UsersHandler, authenticate echo.MiddlewareFunc) {
	group := e.Group("/api/v1/users", authenticate)
	group.GET("", handler.List)
	group.GET("/:id", handler.Get)
	group.PATCH("/update/:id", handler.Update)
	group.POST(
		"/upload/:id",
		handler.UploadAvatar,
		EnsureOwnAccount("You can only upload an avatar for your own account"),
	)
	group.DELETE("/delete/:id", handler.Delete)
}
