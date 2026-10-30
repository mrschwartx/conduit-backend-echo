package application

import (
	"strings"

	sharedomain "taskly/backend/internal/shared/domain"
)

func AssertOwnAccount(requesterID, userID, message string) error {
	if requesterID != strings.ToLower(userID) {
		return sharedomain.NewAuthenticationError(message)
	}
	return nil
}
