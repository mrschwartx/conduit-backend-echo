package application

import (
	"context"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

const taskNotFoundMessage = "Task not found!"

type TaskAccess struct {
	RequesterID string
	TaskID      string
}

type GetTask struct {
	tasks domain.Repository
}

func NewGetTask(tasks domain.Repository) *GetTask {
	return &GetTask{tasks: tasks}
}

func (uc *GetTask) Execute(ctx context.Context, access TaskAccess) (TaskDTO, error) {
	task, err := loadOwnedTask(ctx, uc.tasks, access, "You are not authorized to view this task")
	if err != nil {
		return TaskDTO{}, err
	}
	return ToTaskDTO(task), nil
}

func loadOwnedTask(ctx context.Context, tasks domain.Repository, access TaskAccess, forbiddenMessage string) (domain.Task, error) {
	id, err := sharedomain.ParseEntityID(access.TaskID, "task")
	if err != nil {
		return domain.Task{}, err
	}
	task, found, err := tasks.FindByID(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}
	if !found {
		return domain.Task{}, sharedomain.NewNotFoundError(taskNotFoundMessage)
	}
	if !task.IsOwnedBy(access.RequesterID) {
		return domain.Task{}, sharedomain.NewForbiddenError(forbiddenMessage)
	}
	return task, nil
}
