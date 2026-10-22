package identity

import "conduit/backend/internal/platform/httpx"

// NewUserInput is the "user" object of POST /users.
type NewUserInput struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is the POST /users envelope.
type RegisterRequest struct {
	User *NewUserInput `json:"user"`
}

// LoginInput is the "user" object of POST /users/login.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest is the POST /users/login envelope.
type LoginRequest struct {
	User *LoginInput `json:"user"`
}

// UpdateUserInput is the "user" object of PUT /user. Optional distinguishes an
// absent key from an explicit null.
type UpdateUserInput struct {
	Email    httpx.Optional[string] `json:"email"`
	Username httpx.Optional[string] `json:"username"`
	Password httpx.Optional[string] `json:"password"`
	Bio      httpx.Optional[string] `json:"bio"`
	Image    httpx.Optional[string] `json:"image"`
}

// UpdateUserRequest is the PUT /user envelope.
type UpdateUserRequest struct {
	User *UpdateUserInput `json:"user"`
}

// UserDTO matches the contract "User" schema.
type UserDTO struct {
	Email    string  `json:"email"`
	Token    string  `json:"token"`
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
	Image    *string `json:"image"`
}

// UserResponse is the {"user": ...} envelope.
type UserResponse struct {
	User UserDTO `json:"user"`
}

func toResponse(r *AuthResult) UserResponse {
	return UserResponse{User: UserDTO{
		Email:    r.User.Email,
		Token:    r.Token,
		Username: r.User.Username,
		Bio:      r.User.Bio,
		Image:    r.User.Image,
	}}
}
