package application

import (
	"context"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type DeleteTasksByOwner struct {
	tasks domain.Repository
}

func NewDeleteTasksByOwner(tasks domain.Repository) *DeleteTasksByOwner {
	return &DeleteTasksByOwner{tasks: tasks}
}

func (uc *DeleteTasksByOwner) Execute(ctx context.Context, ownerID string) (int64, error) {
	owner, err := sharedomain.ParseEntityID(ownerID, "user")
	if err != nil {
		return 0, err
	}
	return uc.tasks.DeleteByOwner(ctx, owner)
}
