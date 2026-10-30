package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v5"

	sharedomain "taskly/backend/internal/shared/domain"
)

const defaultErrorMessage = "We're having technical issues. Please try again later."

type ErrorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

type NotFoundResponse struct {
	Message string `json:"message"`
}

var statusByKind = map[sharedomain.ErrorKind]int{
	sharedomain.KindValidation:     http.StatusBadRequest,
	sharedomain.KindNotFound:       http.StatusNotFound,
	sharedomain.KindAlreadyExists:  http.StatusUnprocessableEntity,
	sharedomain.KindAuthentication: http.StatusUnauthorized,
	sharedomain.KindForbidden:      http.StatusForbidden,
}

type outcome int

const (
	outcomeClientError outcome = iota
	outcomeRouteNotFound
	outcomeUnhandled
)

func NewErrorHandler(logger *slog.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		if response, _ := echo.UnwrapResponse(c.Response()); response != nil && response.Committed {
			return
		}

		status, message, kind := classify(err)
		var payload any
		switch kind {
		case outcomeRouteNotFound:
			payload = NotFoundResponse{Message: "Not found"}
		case outcomeClientError:
			logger.Warn("Request rejected", "status", status, "message", message)
			payload = ErrorResponse{Status: status, Message: message}
		default:
			logger.Error("Unhandled error", "message", err.Error())
			payload = ErrorResponse{Status: status, Message: defaultErrorMessage}
		}

		if c.Request().Method == http.MethodHead {
			_ = c.NoContent(status)
			return
		}
		if sendErr := c.JSON(status, payload); sendErr != nil {
			logger.Error("Failed to send error response", "message", sendErr.Error())
		}
	}
}

func classify(err error) (int, string, outcome) {
	var domainErr *sharedomain.Error
	if errors.As(err, &domainErr) {
		if status, known := statusByKind[domainErr.Kind]; known {
			return status, domainErr.Message, outcomeClientError
		}
	}

	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge, payloadTooLargeText, outcomeClientError
	}

	switch code := echo.StatusCode(err); code {
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		return http.StatusNotFound, "", outcomeRouteNotFound
	case http.StatusRequestEntityTooLarge:
		return code, payloadTooLargeText, outcomeClientError
	}
	return http.StatusInternalServerError, "", outcomeUnhandled
}
