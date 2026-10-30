package application

import (
	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

type VerifyAccessToken struct {
	tokens domain.TokenService
}

func NewVerifyAccessToken(tokens domain.TokenService) *VerifyAccessToken {
	return &VerifyAccessToken{tokens: tokens}
}

func (uc *VerifyAccessToken) Execute(token string) (string, error) {
	if token == "" {
		return "", sharedomain.NewAuthenticationError("Unauthorized: Token not provided")
	}
	userID, err := uc.tokens.Verify(token)
	if err != nil {
		return "", sharedomain.NewForbiddenError("Forbidden: Invalid token")
	}
	return userID, nil
}
