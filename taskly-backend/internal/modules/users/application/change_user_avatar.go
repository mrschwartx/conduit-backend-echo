package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type ChangeUserAvatarInput struct {
	RequesterID string
	UserID      string
	AvatarPath  string
}

type ChangeUserAvatar struct {
	users         domain.Repository
	avatarStorage AvatarStorage
	now           func() time.Time
}

func NewChangeUserAvatar(users domain.Repository, avatarStorage AvatarStorage) *ChangeUserAvatar {
	return &ChangeUserAvatar{users: users, avatarStorage: avatarStorage, now: time.Now}
}

func (uc *ChangeUserAvatar) Execute(ctx context.Context, input ChangeUserAvatarInput) (result UserDTO, err error) {
	defer func() {
		if err != nil && input.AvatarPath != "" {
			uc.avatarStorage.Remove(context.WithoutCancel(ctx), input.AvatarPath)
		}
	}()

	if err = AssertOwnAccount(input.RequesterID, input.UserID, "You can only upload an avatar for your own account"); err != nil {
		return UserDTO{}, err
	}
	id, err := sharedomain.ParseEntityID(input.UserID, "user")
	if err != nil {
		return UserDTO{}, err
	}
	if input.AvatarPath == "" {
		return UserDTO{}, sharedomain.NewValidationError("No file uploaded")
	}

	user, found, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return UserDTO{}, err
	}
	if !found {
		return UserDTO{}, sharedomain.NewNotFoundError(userNotFoundMessage)
	}

	previousAvatar := user.Avatar()
	user.ChangeAvatar(input.AvatarPath, uc.now())

	updated, err := uc.users.Update(ctx, user)
	if err != nil {
		return UserDTO{}, err
	}
	if !updated {
		return UserDTO{}, sharedomain.NewNotFoundError(userNotFoundMessage)
	}

	uc.avatarStorage.Remove(context.WithoutCancel(ctx), previousAvatar)
	return ToUserDTO(user), nil
}
