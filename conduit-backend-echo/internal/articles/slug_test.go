package articles

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlugify(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"simple", "Hello World", "hello-world"},
		{"diacritics", "Crème Brûlée à la façon", "creme-brulee-a-la-facon"},
		{"punctuation runs", "  Go -- is   fun!!! ", "go-is-fun"},
		{"digits", "Top 10 Tips", "top-10-tips"},
		{"only symbols", "!!!", "article"},
		{"non latin", "你好", "article"},
		{"empty", "", "article"},
		{"reserved stays base", "Feed", "feed"},
		{"truncate", strings.Repeat("a", 100), strings.Repeat("a", 80)},
		{"truncate trims dash", strings.Repeat("a", 79) + " bbb", strings.Repeat("a", 79)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Slugify(tt.in))
		})
	}
}

func TestSlugCandidate(t *testing.T) {
	assert.Equal(t, "abc", slugCandidate("abc", 1))
	assert.Equal(t, "abc-2", slugCandidate("abc", 2))
	long := strings.Repeat("a", 80)
	got := slugCandidate(long, 12)
	assert.Len(t, got, 80)
	assert.True(t, strings.HasSuffix(got, "-12"))
}

func TestNormalizeTags(t *testing.T) {
	tests := []struct {
		name    string
		in      []string
		want    []string
		wantErr bool
	}{
		{"nil", nil, []string{}, false},
		{"trim and dedupe keep order", []string{" go ", "web", "go", "api"}, []string{"go", "web", "api"}, false},
		{"empty tag", []string{"go", "  "}, nil, true},
		{"space inside", []string{"a b"}, nil, true},
		{"comma", []string{"a,b"}, nil, true},
		{"too long", []string{strings.Repeat("x", 31)}, nil, true},
		{"exactly 30", []string{strings.Repeat("x", 30)}, []string{strings.Repeat("x", 30)}, false},
		{"unicode counted as chars", []string{strings.Repeat("é", 30)}, []string{strings.Repeat("é", 30)}, false},
		{"too many", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}, nil, true},
		{"dupes do not count", []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "10"}, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, msg := normalizeTags(tt.in)
			if tt.wantErr {
				assert.NotEmpty(t, msg)
				return
			}
			assert.Empty(t, msg)
			assert.Equal(t, tt.want, got)
		})
	}
}
