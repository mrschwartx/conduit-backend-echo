package application

import (
	"context"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type GetUser struct {
	users domain.Repository
}

func NewGetUser(users domain.Repository) *GetUser {
	return &GetUser{users: users}
}

func (uc *GetUser) Execute(ctx context.Context, userID string) (UserDTO, error) {
	id, err := sharedomain.ParseEntityID(userID, "user")
	if err != nil {
		return UserDTO{}, err
	}
	user, found, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return UserDTO{}, err
	}
	if !found {
		return UserDTO{}, sharedomain.NewNotFoundError(userNotFoundMessage)
	}
	return ToUserDTO(user), nil
}

const userNotFoundMessage = "User not found!"
