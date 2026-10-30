package application

import (
	"context"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type AuthenticateUserInput struct {
	Email    sharedomain.Raw `json:"email"`
	Password sharedomain.Raw `json:"password"`
}

type AuthenticateUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	tokens domain.TokenService
}

func NewAuthenticateUser(users domain.Repository, hasher domain.PasswordHasher, tokens domain.TokenService) *AuthenticateUser {
	return &AuthenticateUser{users: users, hasher: hasher, tokens: tokens}
}

func (uc *AuthenticateUser) Execute(ctx context.Context, input AuthenticateUserInput) (AuthResult, error) {
	email, emailOK := input.Email.Text()
	password, passwordOK := input.Password.Text()
	if !emailOK || !passwordOK {
		return AuthResult{}, sharedomain.NewValidationError("Email and password are required")
	}

	user, found, err := uc.users.FindByEmail(ctx, sharedomain.TrimText(email))
	if err != nil {
		return AuthResult{}, err
	}
	if !found {
		return AuthResult{}, sharedomain.NewNotFoundError("User not found!")
	}

	if !uc.hasher.Compare(ctx, password, user.PasswordHash()) {
		return AuthResult{}, sharedomain.NewAuthenticationError("Wrong password!")
	}

	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: ToUserDTO(user), Token: token}, nil
}
