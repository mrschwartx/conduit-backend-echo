package users

import (
	"log/slog"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/config"
	"taskly/backend/internal/modules/users/application"
	"taskly/backend/internal/modules/users/infrastructure"
	"taskly/backend/internal/modules/users/presentation/rest"
	sharedapp "taskly/backend/internal/shared/application"
	"taskly/backend/internal/shared/infrastructure/postgres"
)

type Dependencies struct {
	DB     postgres.Querier
	Config config.Config
	Logger *slog.Logger
	Events sharedapp.EventBus
}

type UseCases struct {
	RegisterUser      *application.RegisterUser
	AuthenticateUser  *application.AuthenticateUser
	VerifyAccessToken *application.VerifyAccessToken
	GetUser           *application.GetUser
	ListUsers         *application.ListUsers
	UpdateUser        *application.UpdateUser
	DeleteUser        *application.DeleteUser
	ChangeUserAvatar  *application.ChangeUserAvatar
}

type Module struct {
	UseCases     UseCases
	Authenticate echo.MiddlewareFunc

	authHandler  *rest.AuthHandler
	usersHandler *rest.UsersHandler
}

func NewModule(deps Dependencies) *Module {
	cfg := deps.Config
	repository := infrastructure.NewPostgresRepository(deps.DB)
	hasher := infrastructure.NewBcryptHasher(cfg.Auth.BcryptRounds)
	tokens := infrastructure.NewJWTTokenService(cfg.Auth.Secret, cfg.Auth.TokenTTL)
	avatarStorage := infrastructure.NewLocalAvatarStorage(cfg.Storage.PublicDir, deps.Logger)

	useCases := UseCases{
		RegisterUser:      application.NewRegisterUser(repository, hasher, tokens),
		AuthenticateUser:  application.NewAuthenticateUser(repository, hasher, tokens),
		VerifyAccessToken: application.NewVerifyAccessToken(tokens),
		GetUser:           application.NewGetUser(repository),
		ListUsers:         application.NewListUsers(repository),
		UpdateUser:        application.NewUpdateUser(repository, hasher),
		DeleteUser:        application.NewDeleteUser(repository, avatarStorage, deps.Events),
		ChangeUserAvatar:  application.NewChangeUserAvatar(repository, avatarStorage),
	}

	cookie := rest.NewAuthCookie(cfg.Auth, cfg.App.IsProduction)
	uploader := rest.NewAvatarUploader(cfg.Storage.AvatarsDir, cfg.Storage.AvatarMaxBytes)

	return &Module{
		UseCases:     useCases,
		Authenticate: rest.Authenticate(useCases.VerifyAccessToken, cfg.Auth.CookieName),
		authHandler:  rest.NewAuthHandler(useCases.RegisterUser, useCases.AuthenticateUser, cookie),
		usersHandler: rest.NewUsersHandler(
			useCases.GetUser, useCases.ListUsers, useCases.UpdateUser,
			useCases.DeleteUser, useCases.ChangeUserAvatar, uploader, cookie,
		),
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {
	rest.RegisterAuthRoutes(e, m.authHandler)
	rest.RegisterUsersRoutes(e, m.usersHandler, m.Authenticate)
}
