package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	sharedomain "taskly/backend/internal/shared/domain"
)

type payload struct {
	Name sharedomain.Raw `json:"name"`
}

func invokeHandler(t *testing.T, err error, logs *bytes.Buffer) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), recorder)
	NewErrorHandler(slog.New(slog.NewTextHandler(logs, nil)))(c, err)
	return recorder
}

func TestErrorHandlerMapsDomainErrors(t *testing.T) {
	cases := map[int]error{
		400: sharedomain.NewValidationError("bad"),
		401: sharedomain.NewAuthenticationError("bad"),
		403: sharedomain.NewForbiddenError("bad"),
		404: sharedomain.NewNotFoundError("bad"),
		422: sharedomain.NewAlreadyExistsError("bad"),
	}
	for status, err := range cases {
		var logs bytes.Buffer
		recorder := invokeHandler(t, err, &logs)
		if recorder.Code != status {
			t.Errorf("status = %d, want %d", recorder.Code, status)
		}
		var body ErrorResponse
		if json.Unmarshal(recorder.Body.Bytes(), &body) != nil || body.Status != status || body.Message != "bad" {
			t.Errorf("unexpected body %s", recorder.Body.String())
		}
		if !strings.Contains(logs.String(), "level=WARN") {
			t.Errorf("client errors are logged as warnings: %s", logs.String())
		}
	}
}

func TestErrorHandlerHidesUnexpectedErrors(t *testing.T) {
	var logs bytes.Buffer
	recorder := invokeHandler(t, errors.New("database exploded: secret details"), &logs)

	if recorder.Code != 500 {
		t.Fatalf("status = %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "secret") {
		t.Fatalf("internal details leaked: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), defaultErrorMessage) {
		t.Fatalf("unexpected body %s", recorder.Body.String())
	}
	if !strings.Contains(logs.String(), "level=ERROR") || !strings.Contains(logs.String(), "secret details") {
		t.Fatalf("the cause must be logged: %s", logs.String())
	}
}

func TestErrorHandlerRoutingErrors(t *testing.T) {
	for _, err := range []error{echo.ErrNotFound, echo.ErrMethodNotAllowed} {
		var logs bytes.Buffer
		recorder := invokeHandler(t, err, &logs)
		if recorder.Code != 404 || strings.TrimSpace(recorder.Body.String()) != `{"message":"Not found"}` {
			t.Errorf("unexpected response %d %s", recorder.Code, recorder.Body.String())
		}
		if logs.Len() != 0 {
			t.Errorf("routing misses are not logged: %s", logs.String())
		}
	}

	var logs bytes.Buffer
	recorder := invokeHandler(t, NewPayloadTooLargeError(), &logs)
	if recorder.Code != 413 || !strings.Contains(recorder.Body.String(), "Request body is too large") {
		t.Errorf("unexpected response %d %s", recorder.Code, recorder.Body.String())
	}
}

func TestErrorHandlerSkipsCommittedResponses(t *testing.T) {
	recorder := httptest.NewRecorder()
	c := echo.New().NewContext(httptest.NewRequest(http.MethodGet, "/", nil), recorder)
	_ = c.String(http.StatusOK, "done")

	var logs bytes.Buffer
	NewErrorHandler(slog.New(slog.NewTextHandler(&logs, nil)))(c, errors.New("late"))

	if recorder.Body.String() != "done" {
		t.Fatalf("response was overwritten: %s", recorder.Body.String())
	}
}

func decode(t *testing.T, contentType, body string) (payload, error) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	c := echo.New().NewContext(request, httptest.NewRecorder())
	var result payload
	err := DecodeJSON(c, &result)
	return result, err
}

func TestDecodeJSON(t *testing.T) {
	if result, err := decode(t, "application/json; charset=utf-8", `{"name":"x"}`); err != nil {
		t.Fatal(err)
	} else if text, ok := result.Name.Text(); !ok || text != "x" {
		t.Fatalf("unexpected %+v", result)
	}

	for name, tc := range map[string]struct{ contentType, body string }{
		"no content type": {"", `{"name":"x"}`},
		"form content":    {"text/plain", `{"name":"x"}`},
		"empty body":      {"application/json", ""},
		"whitespace body": {"application/json", "  \n"},
		"array body":      {"application/json", `[1,2]`},
	} {
		t.Run(name, func(t *testing.T) {
			result, err := decode(t, tc.contentType, tc.body)
			if err != nil {
				t.Fatal(err)
			}
			if result.Name.IsSet() && name != "no content type" && name != "form content" {
				t.Fatal("no fields expected")
			}
			if (name == "no content type" || name == "form content") && result.Name.IsSet() {
				t.Fatal("body must be ignored without a JSON content type")
			}
		})
	}

	for _, body := range []string{"{broken", "null", `"text"`, "42", "[1,", `{"name":}`} {
		_, err := decode(t, "application/json", body)
		if !sharedomain.IsKind(err, sharedomain.KindValidation) || err.Error() != "Malformed JSON body" {
			t.Errorf("%q: expected malformed error, got %v", body, err)
		}
	}
}

func TestDecodeJSONLimitsBodySize(t *testing.T) {
	body := `{"name":"` + strings.Repeat("a", MaxJSONBytes) + `"}`
	_, err := decode(t, "application/json", body)
	if echo.StatusCode(err) != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %v", err)
	}
}
