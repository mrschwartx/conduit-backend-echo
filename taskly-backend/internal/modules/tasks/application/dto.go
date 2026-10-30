package application

import (
	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type TaskDTO struct {
	ID          string `json:"_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Due         string `json:"due"`
	Status      string `json:"status"`
	Owner       string `json:"owner"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type TaskListDTO struct {
	Tasks     []TaskDTO `json:"tasks"`
	TaskCount int       `json:"taskCount"`
}

func ToTaskDTO(task domain.Task) TaskDTO {
	return TaskDTO{
		ID:          task.ID(),
		Name:        task.Name().Value(),
		Description: task.Description().Value(),
		Priority:    task.Priority().Value(),
		Due:         sharedomain.FormatTimestamp(task.Due().Value()),
		Status:      task.Status().Value(),
		Owner:       task.OwnerID(),
		CreatedAt:   sharedomain.FormatTimestamp(task.CreatedAt()),
		UpdatedAt:   sharedomain.FormatTimestamp(task.UpdatedAt()),
	}
}
