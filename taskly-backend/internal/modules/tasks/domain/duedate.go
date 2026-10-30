package domain

import (
	"time"

	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	minDueYear = 1
	maxDueYear = 9999
)

var dueDateLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04Z07:00",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04",
	"2006-01-02",
	"2006-01",
	"2006",
	time.RFC1123Z,
	time.RFC1123,
	time.RFC850,
	time.ANSIC,
}

type DueDate struct {
	value time.Time
}

func NewDueDate(raw sharedomain.Raw) (DueDate, error) {
	text, ok := raw.Text()
	trimmed := sharedomain.TrimText(text)
	if !ok || trimmed == "" {
		return DueDate{}, sharedomain.NewValidationError("Task due date is required")
	}

	parsed, valid := parseDueDate(trimmed)
	if !valid || parsed.Year() < minDueYear || parsed.Year() > maxDueYear {
		return DueDate{}, sharedomain.NewValidationError("Task due date is invalid")
	}
	return DueDate{value: sharedomain.TruncateToMillisecond(parsed)}, nil
}

func RestoreDueDate(value time.Time) DueDate {
	return DueDate{value: value.UTC()}
}

func (d DueDate) Value() time.Time {
	return d.value
}

func (d DueDate) Equals(other DueDate) bool {
	return d.value.Equal(other.value)
}

func parseDueDate(text string) (time.Time, bool) {
	for _, layout := range dueDateLayouts {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}
