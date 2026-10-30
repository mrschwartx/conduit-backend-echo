package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/labstack/echo/v5"

	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	MaxJSONBytes        = 100 * 1024
	malformedJSONText   = "Malformed JSON body"
	payloadTooLargeText = "Request body is too large"
)

func NewPayloadTooLargeError() error {
	return echo.NewHTTPError(http.StatusRequestEntityTooLarge, payloadTooLargeText)
}

func DecodeJSON(c *echo.Context, destination any) error {
	request := c.Request()
	mediaType, _, err := mime.ParseMediaType(request.Header.Get(echo.HeaderContentType))
	if err != nil || mediaType != echo.MIMEApplicationJSON {
		return nil
	}

	body, err := io.ReadAll(http.MaxBytesReader(c.Response(), request.Body, MaxJSONBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return NewPayloadTooLargeError()
		}
		return sharedomain.NewValidationError(malformedJSONText)
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	switch trimmed[0] {
	case '[':
		if !json.Valid(trimmed) {
			return sharedomain.NewValidationError(malformedJSONText)
		}
		return nil
	case '{':
		if err := json.Unmarshal(trimmed, destination); err != nil {
			return sharedomain.NewValidationError(malformedJSONText)
		}
		return nil
	default:
		return sharedomain.NewValidationError(malformedJSONText)
	}
}
