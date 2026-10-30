package application

import (
	"context"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type ListUsersInput struct {
	Username []string
	Email    []string
}

type ListUsers struct {
	users domain.Repository
}

func NewListUsers(users domain.Repository) *ListUsers {
	return &ListUsers{users: users}
}

func (uc *ListUsers) Execute(ctx context.Context, input ListUsersInput) ([]UserDTO, error) {
	username, err := optionalSingleValue(input.Username, "Username")
	if err != nil {
		return nil, err
	}
	email, err := optionalSingleValue(input.Email, "Email")
	if err != nil {
		return nil, err
	}

	users, err := uc.users.Search(ctx, domain.SearchFilter{Username: username, Email: email})
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		return nil, sharedomain.NewNotFoundError("No users found")
	}

	result := make([]UserDTO, 0, len(users))
	for _, user := range users {
		result = append(result, ToUserDTO(user))
	}
	return result, nil
}

func optionalSingleValue(values []string, label string) (string, error) {
	switch len(values) {
	case 0:
		return "", nil
	case 1:
		return values[0], nil
	default:
		return "", sharedomain.NewValidationError(label + " filter must be a string")
	}
}
