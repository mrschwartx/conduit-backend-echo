package application

import (
	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type UserDTO struct {
	ID        string `json:"_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Avatar    string `json:"avatar"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func ToUserDTO(user domain.User) UserDTO {
	return UserDTO{
		ID:        user.ID(),
		Username:  user.Username().Value(),
		Email:     user.Email().Value(),
		Avatar:    user.Avatar(),
		CreatedAt: sharedomain.FormatTimestamp(user.CreatedAt()),
		UpdatedAt: sharedomain.FormatTimestamp(user.UpdatedAt()),
	}
}

type AuthResult struct {
	User  UserDTO
	Token string
}
