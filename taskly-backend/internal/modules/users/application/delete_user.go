package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/users/domain"
	sharedapp "taskly/backend/internal/shared/application"
	sharedomain "taskly/backend/internal/shared/domain"
)

type DeleteUserInput struct {
	RequesterID string
	UserID      string
}

type DeleteUser struct {
	users         domain.Repository
	avatarStorage AvatarStorage
	events        sharedapp.EventBus
	now           func() time.Time
}

func NewDeleteUser(users domain.Repository, avatarStorage AvatarStorage, events sharedapp.EventBus) *DeleteUser {
	return &DeleteUser{users: users, avatarStorage: avatarStorage, events: events, now: time.Now}
}

func (uc *DeleteUser) Execute(ctx context.Context, input DeleteUserInput) error {
	if err := AssertOwnAccount(input.RequesterID, input.UserID, "You can only delete your own account"); err != nil {
		return err
	}
	id, err := sharedomain.ParseEntityID(input.UserID, "user")
	if err != nil {
		return err
	}

	user, found, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return sharedomain.NewNotFoundError(userNotFoundMessage)
	}

	deleted, err := uc.users.DeleteByID(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return sharedomain.NewNotFoundError(userNotFoundMessage)
	}

	detached := context.WithoutCancel(ctx)
	uc.avatarStorage.Remove(detached, user.Avatar())
	uc.events.Publish(detached, sharedomain.NewEvent(
		sharedomain.UserDeletedEvent,
		sharedomain.UserDeleted{UserID: id},
		uc.now(),
	))
	return nil
}
