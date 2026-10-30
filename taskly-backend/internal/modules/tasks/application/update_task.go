package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type UpdateTaskInput struct {
	Access  TaskAccess
	Changes TaskInput
}

type UpdateTask struct {
	tasks domain.Repository
	now   func() time.Time
}

func NewUpdateTask(tasks domain.Repository) *UpdateTask {
	return &UpdateTask{tasks: tasks, now: time.Now}
}

func (uc *UpdateTask) Execute(ctx context.Context, input UpdateTaskInput) (TaskDTO, error) {
	task, err := loadOwnedTask(ctx, uc.tasks, input.Access, "You are not authorized to update this task")
	if err != nil {
		return TaskDTO{}, err
	}

	changes, err := parseChanges(input.Changes)
	if err != nil {
		return TaskDTO{}, err
	}
	task.Update(changes, uc.now())

	updated, err := uc.tasks.Update(ctx, task)
	if err != nil {
		return TaskDTO{}, err
	}
	if !updated {
		return TaskDTO{}, sharedomain.NewNotFoundError(taskNotFoundMessage)
	}
	return ToTaskDTO(task), nil
}

func parseChanges(input TaskInput) (domain.Changes, error) {
	var changes domain.Changes
	if input.Name.IsSet() {
		value, err := domain.NewTaskName(input.Name)
		if err != nil {
			return changes, err
		}
		changes.Name = &value
	}
	if input.Description.IsSet() {
		value, err := domain.NewTaskDescription(input.Description)
		if err != nil {
			return changes, err
		}
		changes.Description = &value
	}
	if input.Priority.IsSet() {
		value, err := domain.NewTaskPriority(input.Priority)
		if err != nil {
			return changes, err
		}
		changes.Priority = &value
	}
	if input.Due.IsSet() {
		value, err := domain.NewDueDate(input.Due)
		if err != nil {
			return changes, err
		}
		changes.Due = &value
	}
	if input.Status.IsSet() {
		value, err := domain.NewTaskStatus(input.Status)
		if err != nil {
			return changes, err
		}
		changes.Status = &value
	}
	if changes.IsEmpty() {
		return changes, sharedomain.NewValidationError("No updatable fields provided")
	}
	return changes, nil
}
