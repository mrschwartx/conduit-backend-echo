package bootstrap

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"

	"taskly/backend/internal/config"
	"taskly/backend/internal/modules/tasks"
	"taskly/backend/internal/modules/users"
	"taskly/backend/internal/shared/infrastructure/events"
	"taskly/backend/internal/shared/infrastructure/postgres"
)

type Container struct {
	Config config.Config
	Logger *slog.Logger
	Pool   *pgxpool.Pool
	Events *events.InMemoryBus
	Users  *users.Module
	Tasks  *tasks.Module
}

func NewContainer(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Container, error) {
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{
		URL:     cfg.Database.URL,
		PoolMax: cfg.Database.PoolMax,
		Timeout: cfg.Database.Timeout,
	})
	if err != nil {
		return nil, err
	}

	if _, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), logger); err != nil {
		pool.Close()
		return nil, err
	}

	bus := events.NewInMemoryBus(logger)
	usersModule := users.NewModule(users.Dependencies{
		DB:     pool,
		Config: cfg,
		Logger: logger,
		Events: bus,
	})
	tasksModule := tasks.NewModule(tasks.Dependencies{
		DB:           pool,
		Events:       bus,
		Authenticate: usersModule.Authenticate,
	})

	return &Container{
		Config: cfg,
		Logger: logger,
		Pool:   pool,
		Events: bus,
		Users:  usersModule,
		Tasks:  tasksModule,
	}, nil
}

func (c *Container) Close() {
	c.Pool.Close()
}
