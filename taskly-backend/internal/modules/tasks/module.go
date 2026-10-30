package tasks

import (
	"context"

	"github.com/labstack/echo/v5"

	"taskly/backend/internal/modules/tasks/application"
	"taskly/backend/internal/modules/tasks/infrastructure"
	"taskly/backend/internal/modules/tasks/presentation/rest"
	sharedapp "taskly/backend/internal/shared/application"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/postgres"
)

type Dependencies struct {
	DB           postgres.Querier
	Events       sharedapp.EventBus
	Authenticate echo.MiddlewareFunc
}

type UseCases struct {
	CreateTask         *application.CreateTask
	GetTask            *application.GetTask
	ListTasksByUser    *application.ListTasksByUser
	UpdateTask         *application.UpdateTask
	DeleteTask         *application.DeleteTask
	DeleteTasksByOwner *application.DeleteTasksByOwner
}

type Module struct {
	UseCases UseCases

	handler      *rest.Handler
	authenticate echo.MiddlewareFunc
}

func NewModule(deps Dependencies) *Module {
	repository := infrastructure.NewPostgresRepository(deps.DB)

	useCases := UseCases{
		CreateTask:         application.NewCreateTask(repository),
		GetTask:            application.NewGetTask(repository),
		ListTasksByUser:    application.NewListTasksByUser(repository),
		UpdateTask:         application.NewUpdateTask(repository),
		DeleteTask:         application.NewDeleteTask(repository),
		DeleteTasksByOwner: application.NewDeleteTasksByOwner(repository),
	}

	deps.Events.Subscribe(sharedomain.UserDeletedEvent, func(ctx context.Context, event sharedomain.Event) error {
		payload, ok := event.Payload.(sharedomain.UserDeleted)
		if !ok {
			return nil
		}
		_, err := useCases.DeleteTasksByOwner.Execute(ctx, payload.UserID)
		return err
	})

	return &Module{
		UseCases: useCases,
		handler: rest.NewHandler(
			useCases.CreateTask, useCases.GetTask, useCases.ListTasksByUser,
			useCases.UpdateTask, useCases.DeleteTask,
		),
		authenticate: deps.Authenticate,
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {
	rest.RegisterRoutes(e, m.handler, m.authenticate)
}
