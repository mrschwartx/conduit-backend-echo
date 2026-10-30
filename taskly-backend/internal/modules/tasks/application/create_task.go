package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type TaskInput struct {
	Name        sharedomain.Raw `json:"name"`
	Description sharedomain.Raw `json:"description"`
	Priority    sharedomain.Raw `json:"priority"`
	Due         sharedomain.Raw `json:"due"`
	Status      sharedomain.Raw `json:"status"`
}

type CreateTask struct {
	tasks domain.Repository
	now   func() time.Time
}

func NewCreateTask(tasks domain.Repository) *CreateTask {
	return &CreateTask{tasks: tasks, now: time.Now}
}

func (uc *CreateTask) Execute(ctx context.Context, ownerID string, input TaskInput) (TaskDTO, error) {
	name, err := domain.NewTaskName(input.Name)
	if err != nil {
		return TaskDTO{}, err
	}
	description, err := domain.NewTaskDescription(input.Description)
	if err != nil {
		return TaskDTO{}, err
	}
	priority, err := domain.NewTaskPriority(input.Priority)
	if err != nil {
		return TaskDTO{}, err
	}
	due, err := domain.NewDueDate(input.Due)
	if err != nil {
		return TaskDTO{}, err
	}
	status, err := domain.NewTaskStatus(input.Status)
	if err != nil {
		return TaskDTO{}, err
	}

	task := domain.Create(domain.NewTask{
		ID:          uc.tasks.NextID(),
		OwnerID:     ownerID,
		Name:        name,
		Description: description,
		Priority:    priority,
		Due:         due,
		Status:      status,
		Now:         uc.now(),
	})
	if err := uc.tasks.Add(ctx, task); err != nil {
		return TaskDTO{}, err
	}
	return ToTaskDTO(task), nil
}
