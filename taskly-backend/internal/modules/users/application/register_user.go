package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type RegisterUserInput struct {
	Username sharedomain.Raw `json:"username"`
	Email    sharedomain.Raw `json:"email"`
	Password sharedomain.Raw `json:"password"`
}

type RegisterUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	tokens domain.TokenService
	now    func() time.Time
}

func NewRegisterUser(users domain.Repository, hasher domain.PasswordHasher, tokens domain.TokenService) *RegisterUser {
	return &RegisterUser{users: users, hasher: hasher, tokens: tokens, now: time.Now}
}

func (uc *RegisterUser) Execute(ctx context.Context, input RegisterUserInput) (AuthResult, error) {
	username, err := domain.NewUsername(input.Username)
	if err != nil {
		return AuthResult{}, err
	}
	email, err := domain.NewEmail(input.Email)
	if err != nil {
		return AuthResult{}, err
	}
	password, err := domain.NewPlainPassword(input.Password)
	if err != nil {
		return AuthResult{}, err
	}

	registered, err := uc.users.ExistsByEmailOrUsername(ctx, domain.UniquenessQuery{
		Email:    email.Value(),
		Username: username.Value(),
	})
	if err != nil {
		return AuthResult{}, err
	}
	if registered {
		return AuthResult{}, sharedomain.NewAlreadyExistsError(duplicateMessage)
	}

	passwordHash, err := uc.hasher.Hash(ctx, password.Value())
	if err != nil {
		return AuthResult{}, err
	}

	user := domain.Register(domain.Registration{
		ID:           uc.users.NextID(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Now:          uc.now(),
	})
	if err := uc.users.Add(ctx, user); err != nil {
		return AuthResult{}, err
	}

	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return AuthResult{}, err
	}
	return AuthResult{User: ToUserDTO(user), Token: token}, nil
}

const duplicateMessage = "Email or Username is already registered."
