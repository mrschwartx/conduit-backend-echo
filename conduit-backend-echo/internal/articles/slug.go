package articles

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	maxSlugLen   = 80
	reservedSlug = "feed" // would collide with GET /articles/feed
	maxTags      = 10
	maxTagLen    = 30
)

// Slugify implements the contract slug rules: Unicode NFKD, strip diacritics,
// lowercase, runs of non [a-z0-9] become "-", trim "-", max 80, fallback "article".
func Slugify(title string) string {
	var b strings.Builder
	lastDash := true // suppresses a leading dash
	for _, r := range norm.NFKD.String(title) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		r = unicode.ToLower(r)
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	out := strings.TrimRight(b.String(), "-")
	if len(out) > maxSlugLen { // ASCII only: bytes == characters
		out = strings.TrimRight(out[:maxSlugLen], "-")
	}
	if out == "" {
		return "article"
	}
	return out
}

// slugCandidate returns base for n=1 and base-n for n>=2, never exceeding 80 chars.
func slugCandidate(base string, n int) string {
	if n <= 1 {
		return base
	}
	suffix := "-" + strconv.Itoa(n)
	if limit := maxSlugLen - len(suffix); len(base) > limit {
		base = strings.TrimRight(base[:limit], "-")
	}
	return base + suffix
}

// normalizeTags trims, de-duplicates (keeping first-seen order) and validates
// tags. It returns a message for the "tagList" field when invalid.
func normalizeTags(raw []string) ([]string, string) {
	seen := make(map[string]struct{}, len(raw))
	tags := make([]string, 0, len(raw))
	for _, t := range raw {
		t = strings.TrimSpace(t)
		n := utf8.RuneCountInString(t)
		if n < 1 || n > maxTagLen || strings.ContainsRune(t, ',') || strings.IndexFunc(t, unicode.IsSpace) >= 0 {
			return nil, "contains an invalid tag (1-30 characters, no spaces or commas)"
		}
		if _, dup := seen[t]; dup {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}
	if len(tags) > maxTags {
		return nil, "has too many tags (maximum is 10)"
	}
	return tags, ""
}
