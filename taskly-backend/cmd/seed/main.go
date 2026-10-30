package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"

	"taskly/backend/internal/bootstrap"
	"taskly/backend/internal/config"
	tasksapp "taskly/backend/internal/modules/tasks/application"
	usersapp "taskly/backend/internal/modules/users/application"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/logging"
)

type seedUser struct {
	username string
	email    string
	password string
	tasks    []seedTask
}

type seedTask struct {
	name        string
	description string
	priority    string
	status      string
}

var seeds = []seedUser{
	{
		username: "johndoe123",
		email:    "johndoe123@mail.com",
		password: "password123",
		tasks: []seedTask{{
			name:        "Read Atomic Habits",
			description: "Finish reading Atomic Habits by James Clear",
			priority:    "low",
			status:      "todo",
		}},
	},
	{
		username: "jane78",
		email:    "jane@mail.com",
		password: "securepassword456",
		tasks: []seedTask{{
			name:        "Learn MERN Stack",
			description: "Learn the MERN stack and build a full-stack application with it",
			priority:    "high",
			status:      "todo",
		}},
	},
}

func main() {
	os.Exit(run())
}

func run() int {
	_ = godotenv.Load()
	ctx := context.Background()

	cfg, err := config.Load(os.Getenv)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	logger, closeLogger, err := logging.New(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	defer closeLogger()

	if cfg.App.IsProduction && os.Getenv("SEED_ALLOW_PRODUCTION") != "true" {
		logger.Error("Refusing to seed a production database")
		fmt.Fprintln(os.Stderr, "Refusing to seed a production database")
		return 1
	}

	container, err := bootstrap.NewContainer(ctx, cfg, logger)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
		return 1
	}
	defer container.Close()

	due := time.Now().UTC().Format(time.RFC3339Nano)
	for _, entry := range seeds {
		if err := seedOne(ctx, container, entry, due); err != nil {
			if sharedomain.IsKind(err, sharedomain.KindAlreadyExists) {
				logger.Warn("Skipped existing user " + entry.username)
				continue
			}
			fmt.Fprintln(os.Stderr, "Seeding failed: "+err.Error())
			return 1
		}
		logger.Info("Seeded user " + entry.username)
	}
	logger.Info("Seeding completed")
	return 0
}

func seedOne(ctx context.Context, container *bootstrap.Container, entry seedUser, due string) error {
	result, err := container.Users.UseCases.RegisterUser.Execute(ctx, usersapp.RegisterUserInput{
		Username: sharedomain.RawString(entry.username),
		Email:    sharedomain.RawString(entry.email),
		Password: sharedomain.RawString(entry.password),
	})
	if err != nil {
		return err
	}
	for _, task := range entry.tasks {
		_, err := container.Tasks.UseCases.CreateTask.Execute(ctx, result.User.ID, tasksapp.TaskInput{
			Name:        sharedomain.RawString(task.name),
			Description: sharedomain.RawString(task.description),
			Priority:    sharedomain.RawString(task.priority),
			Due:         sharedomain.RawString(due),
			Status:      sharedomain.RawString(task.status),
		})
		if err != nil {
			return err
		}
	}
	return nil
}
