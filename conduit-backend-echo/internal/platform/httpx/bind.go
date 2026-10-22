package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v5"
)

// Pagination limits.
const (
	DefaultLimit = 20
	MaxLimit     = 50
)

// Bind decodes the JSON request body into dst (see DecodeJSON).
func Bind(c *echo.Context, dst any) error {
	return DecodeJSON(c.Request(), dst)
}

// DecodeJSON strictly decodes a JSON body:
//   - no body                      -> 400
//   - body with non-JSON media type -> 415
//   - malformed JSON / trailing data -> 400
//   - wrong JSON type for a field    -> 422 (errors.<field> = ["is invalid"])
func DecodeJSON(r *http.Request, dst any) error {
	if r.Body == nil || r.Body == http.NoBody || r.ContentLength == 0 {
		return BadRequest("body", "is not valid JSON")
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		return UnsupportedMediaType()
	}

	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return mapDecodeError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return BadRequest("body", "is not valid JSON")
	}
	return nil
}

func mapDecodeError(err error) error {
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return PayloadTooLarge()
	}
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		field := ute.Field
		if i := strings.LastIndex(field, "."); i >= 0 {
			field = field[i+1:]
		}
		if field == "" {
			field = "body"
		}
		return ValidationField(field, "is invalid")
	}
	return BadRequest("body", "is not valid JSON")
}

// ParsePagination validates offset/limit. Empty values use defaults; limit is
// clamped to MaxLimit; non-integers and out-of-range values produce errors.
func ParsePagination(offsetStr, limitStr string) (offset, limit int, errs map[string][]string) {
	errs = map[string][]string{}
	offset, limit = 0, DefaultLimit

	if offsetStr != "" {
		n, err := strconv.Atoi(offsetStr)
		if err != nil || n < 0 {
			errs["offset"] = []string{"must be an integer greater than or equal to 0"}
		} else {
			offset = n
		}
	}
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 {
			errs["limit"] = []string{"must be an integer greater than or equal to 1"}
		} else {
			limit = min(n, MaxLimit)
		}
	}
	return offset, limit, errs
}
