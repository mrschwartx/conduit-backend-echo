package application

import (
	"context"
	"time"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type UpdateUserInput struct {
	RequesterID string
	UserID      string
	Changes     UserChanges
}

type UserChanges struct {
	Username sharedomain.Raw `json:"username"`
	Email    sharedomain.Raw `json:"email"`
	Password sharedomain.Raw `json:"password"`
}

type UpdateUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	now    func() time.Time
}

func NewUpdateUser(users domain.Repository, hasher domain.PasswordHasher) *UpdateUser {
	return &UpdateUser{users: users, hasher: hasher, now: time.Now}
}

func (uc *UpdateUser) Execute(ctx context.Context, input UpdateUserInput) (UserDTO, error) {
	if err := AssertOwnAccount(input.RequesterID, input.UserID, "You can only update your own account"); err != nil {
		return UserDTO{}, err
	}
	id, err := sharedomain.ParseEntityID(input.UserID, "user")
	if err != nil {
		return UserDTO{}, err
	}

	changes := input.Changes
	if !changes.Username.IsSet() && !changes.Email.IsSet() && !changes.Password.IsSet() {
		return UserDTO{}, sharedomain.NewValidationError("No updatable fields provided")
	}

	var username *domain.Username
	if changes.Username.IsSet() {
		value, err := domain.NewUsername(changes.Username)
		if err != nil {
			return UserDTO{}, err
		}
		username = &value
	}
	var email *domain.Email
	if changes.Email.IsSet() {
		value, err := domain.NewEmail(changes.Email)
		if err != nil {
			return UserDTO{}, err
		}
		email = &value
	}
	var password *domain.PlainPassword
	if changes.Password.IsSet() {
		value, err := domain.NewPlainPassword(changes.Password)
		if err != nil {
			return UserDTO{}, err
		}
		password = &value
	}

	user, found, err := uc.users.FindByID(ctx, id)
	if err != nil {
		return UserDTO{}, err
	}
	if !found {
		return UserDTO{}, sharedomain.NewNotFoundError(userNotFoundMessage)
	}

	if username != nil || email != nil {
		query := domain.UniquenessQuery{ExcludeID: id}
		if username != nil {
			query.Username = username.Value()
		}
		if email != nil {
			query.Email = email.Value()
		}
		taken, err := uc.users.ExistsByEmailOrUsername(ctx, query)
		if err != nil {
			return UserDTO{}, err
		}
		if taken {
			return UserDTO{}, sharedomain.NewAlreadyExistsError(duplicateMessage)
		}
	}

	var passwordHash *string
	if password != nil {
		hashed, err := uc.hasher.Hash(ctx, password.Value())
		if err != nil {
			return UserDTO{}, err
		}
		passwordHash = &hashed
	}

	user.UpdateProfile(domain.ProfileChanges{Username: username, Email: email, PasswordHash: passwordHash}, uc.now())

	updated, err := uc.users.Update(ctx, user)
	if err != nil {
		return UserDTO{}, err
	}
	if !updated {
		return UserDTO{}, sharedomain.NewNotFoundError(userNotFoundMessage)
	}
	return ToUserDTO(user), nil
}
