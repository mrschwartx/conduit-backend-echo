package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type patch struct {
	Bio  Optional[string] `json:"bio"`
	Name Optional[string] `json:"name"`
}

type envelope struct {
	P *patch `json:"p"`
}

func TestOptionalDistinguishesAbsentNullValue(t *testing.T) {
	var p patch
	require.NoError(t, json.Unmarshal([]byte(`{"bio":null}`), &p))
	assert.True(t, p.Bio.Set)
	assert.True(t, p.Bio.Null)
	assert.False(t, p.Name.Set, "absent key must stay unset")

	var q patch
	require.NoError(t, json.Unmarshal([]byte(`{"bio":"hi","name":""}`), &q))
	assert.True(t, q.Bio.Set)
	assert.False(t, q.Bio.Null)
	assert.Equal(t, "hi", q.Bio.Value)
	assert.True(t, q.Name.Set)
	assert.Equal(t, "", q.Name.Value)
}

func newReq(body, contentType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}
	return r
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name   string
		body   string
		ct     string
		status int // 0 = ok
		field  string
	}{
		{"ok", `{"p":{"bio":"x"}}`, "application/json", 0, ""},
		{"ok with charset", `{"p":{}}`, "application/json; charset=utf-8", 0, ""},
		{"wrong content type", `{"p":{}}`, "text/plain", 415, "body"},
		{"missing content type", `{"p":{}}`, "", 415, "body"},
		{"malformed", `{"p":`, "application/json", 400, "body"},
		{"trailing data", `{"p":{}} {}`, "application/json", 400, "body"},
		{"empty body", ``, "application/json", 400, "body"},
		{"type mismatch", `{"p":{"bio":123}}`, "application/json", 422, "bio"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var env envelope
			err := DecodeJSON(newReq(tt.body, tt.ct), &env)
			if tt.status == 0 {
				require.NoError(t, err)
				return
			}
			status, errs := Resolve(err)
			assert.Equal(t, tt.status, status)
			assert.Contains(t, errs, tt.field)
		})
	}
}

func TestParsePagination(t *testing.T) {
	tests := []struct {
		name          string
		offset, limit string
		wantOffset    int
		wantLimit     int
		wantErrKeys   []string
	}{
		{"defaults", "", "", 0, 20, nil},
		{"values", "5", "10", 5, 10, nil},
		{"clamped", "0", "500", 0, 50, nil},
		{"limit zero", "", "0", 0, 20, []string{"limit"}},
		{"negative offset", "-1", "", 0, 20, []string{"offset"}},
		{"not integer", "abc", "1.5", 0, 20, []string{"offset", "limit"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, l, errs := ParsePagination(tt.offset, tt.limit)
			assert.Equal(t, tt.wantOffset, o)
			assert.Equal(t, tt.wantLimit, l)
			if len(tt.wantErrKeys) == 0 {
				assert.Empty(t, errs)
				return
			}
			for _, k := range tt.wantErrKeys {
				assert.Contains(t, errs, k)
			}
		})
	}
}

func TestFormatTime(t *testing.T) {
	ts := mustParse(t, "2026-10-21T10:15:30.123456+02:00")
	assert.Equal(t, "2026-10-21T08:15:30.123Z", FormatTime(ts))
}

func mustParse(t *testing.T, v string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, v)
	require.NoError(t, err)
	return ts
}
