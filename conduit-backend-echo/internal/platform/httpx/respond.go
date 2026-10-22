package httpx

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
)

// WriteJSON writes v as a JSON response.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

// WriteErrors writes the contract error envelope.
func WriteErrors(w http.ResponseWriter, status int, errs map[string][]string) {
	WriteJSON(w, status, map[string]any{"errors": errs})
}

// JSON is the handler-facing helper.
func JSON(c *echo.Context, status int, v any) error {
	WriteJSON(c.Response(), status, v)
	return nil
}

// NoContent writes a bodyless 204.
func NoContent(c *echo.Context) error {
	c.Response().WriteHeader(http.StatusNoContent)
	return nil
}

// FormatTime renders a timestamp as UTC ISO-8601 with millisecond precision.
func FormatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
