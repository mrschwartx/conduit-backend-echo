package application

import (
	"context"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type DeleteTask struct {
	tasks domain.Repository
}

func NewDeleteTask(tasks domain.Repository) *DeleteTask {
	return &DeleteTask{tasks: tasks}
}

func (uc *DeleteTask) Execute(ctx context.Context, access TaskAccess) error {
	task, err := loadOwnedTask(ctx, uc.tasks, access, "You are not authorized to delete this task")
	if err != nil {
		return err
	}
	deleted, err := uc.tasks.DeleteByID(ctx, task.ID())
	if err != nil {
		return err
	}
	if !deleted {
		return sharedomain.NewNotFoundError(taskNotFoundMessage)
	}
	return nil
}
