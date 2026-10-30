package domain

import (
	"strings"
	"testing"
	"time"

	sharedomain "taskly/backend/internal/shared/domain"
)

func message(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestValueObjectsApplyDefaultsAndRules(t *testing.T) {
	if value, _ := NewTaskDescription(sharedomain.Raw{}); value.Value() != "" {
		t.Error("description defaults to empty")
	}
	if value, _ := NewTaskStatus(sharedomain.Raw{}); value.Value() != "todo" {
		t.Error("status defaults to todo")
	}

	cases := []struct {
		name string
		call func() error
		want string
	}{
		{"name required", func() error { _, err := NewTaskName(sharedomain.Raw{}); return err }, "Task name is required"},
		{"name empty", func() error { _, err := NewTaskName(sharedomain.RawString("  ")); return err }, "Task name must be between 1 and 120 characters"},
		{"name long", func() error { _, err := NewTaskName(sharedomain.RawString(strings.Repeat("a", 121))); return err }, "Task name must be between 1 and 120 characters"},
		{"description null", func() error { _, err := NewTaskDescription(sharedomain.RawOf(nil)); return err }, "Task description is required"},
		{"description long", func() error {
			_, err := NewTaskDescription(sharedomain.RawString(strings.Repeat("a", 2001)))
			return err
		}, "Task description must be between 0 and 2000 characters"},
		{"priority required", func() error { _, err := NewTaskPriority(sharedomain.Raw{}); return err }, "Task priority is required"},
		{"priority invalid", func() error { _, err := NewTaskPriority(sharedomain.RawString("urgent")); return err }, "Task priority must be one of: low, medium, high"},
		{"status invalid", func() error { _, err := NewTaskStatus(sharedomain.RawString("archived")); return err }, "Task status must be one of: todo, in_progress, done"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := message(tc.call()); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDueDateParsing(t *testing.T) {
	valid := map[string]string{
		"2026-12-08T10:00:00Z":          "2026-12-08T10:00:00.000Z",
		"2026-12-08T10:00:00.123456Z":   "2026-12-08T10:00:00.123Z",
		"2026-12-08T17:00:00+07:00":     "2026-12-08T10:00:00.000Z",
		"2026-12-08T10:00Z":             "2026-12-08T10:00:00.000Z",
		"2026-12-08T10:00:00":           "2026-12-08T10:00:00.000Z",
		"2026-12-08":                    "2026-12-08T00:00:00.000Z",
		"  2026-12-08  ":                "2026-12-08T00:00:00.000Z",
		"2026-12":                       "2026-12-01T00:00:00.000Z",
		"2026":                          "2026-01-01T00:00:00.000Z",
		"Tue, 08 Dec 2026 10:00:00 GMT": "2026-12-08T10:00:00.000Z",
	}
	for input, want := range valid {
		due, err := NewDueDate(sharedomain.RawString(input))
		if err != nil {
			t.Errorf("%q: %v", input, err)
			continue
		}
		if got := sharedomain.FormatTimestamp(due.Value()); got != want {
			t.Errorf("%q = %s, want %s", input, got, want)
		}
	}

	invalid := []string{"someday", "never", "2026-13-01", "2026-02-30", "0000-01-01T00:00:00Z", "10000-01-01"}
	for _, input := range invalid {
		if _, err := NewDueDate(sharedomain.RawString(input)); message(err) != "Task due date is invalid" {
			t.Errorf("%q should be invalid, got %q", input, message(err))
		}
	}
	for _, raw := range []sharedomain.Raw{{}, sharedomain.RawString(""), sharedomain.RawOf(5), sharedomain.RawOf(nil)} {
		if _, err := NewDueDate(raw); message(err) != "Task due date is required" {
			t.Errorf("%v should be required, got %q", raw, message(err))
		}
	}
}

func buildTask(t *testing.T) Task {
	t.Helper()
	name, _ := NewTaskName(sharedomain.RawString("Write tests"))
	description, _ := NewTaskDescription(sharedomain.Raw{})
	priority, _ := NewTaskPriority(sharedomain.RawString("high"))
	due, _ := NewDueDate(sharedomain.RawString("2026-12-08"))
	status, _ := NewTaskStatus(sharedomain.Raw{})
	return Create(NewTask{
		ID: "task-1", OwnerID: "owner-1", Name: name, Description: description,
		Priority: priority, Due: due, Status: status, Now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
}

func TestTaskLifecycle(t *testing.T) {
	task := buildTask(t)
	if !task.IsOwnedBy("owner-1") || task.IsOwnedBy("someone") {
		t.Fatal("ownership check failed")
	}
	if task.Status().Value() != "todo" || !task.CreatedAt().Equal(task.UpdatedAt()) {
		t.Fatalf("unexpected new task: %+v", task.Snapshot())
	}

	name, _ := NewTaskName(sharedomain.RawString("Renamed"))
	status, _ := NewTaskStatus(sharedomain.RawString("done"))
	later := task.CreatedAt().Add(time.Hour)
	task.Update(Changes{Name: &name, Status: &status}, later)

	if task.Name().Value() != "Renamed" || task.Status().Value() != "done" || task.Priority().Value() != "high" {
		t.Fatalf("unexpected update: %+v", task.Snapshot())
	}
	if !task.UpdatedAt().Equal(later) || task.OwnerID() != "owner-1" {
		t.Fatalf("unexpected metadata: %+v", task.Snapshot())
	}
	restored := Restore(task.Snapshot())
	if restored.Snapshot() != task.Snapshot() {
		t.Fatal("restore must round trip")
	}
	if !(Changes{}).IsEmpty() || (Changes{Name: &name}).IsEmpty() {
		t.Fatal("IsEmpty mismatch")
	}
}
