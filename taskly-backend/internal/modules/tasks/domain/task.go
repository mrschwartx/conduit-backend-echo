package domain

import (
	"time"

	sharedomain "taskly/backend/internal/shared/domain"
)

type Task struct {
	id          string
	ownerID     string
	name        TaskName
	description TaskDescription
	priority    TaskPriority
	due         DueDate
	status      TaskStatus
	createdAt   time.Time
	updatedAt   time.Time
}

type Snapshot struct {
	ID          string
	OwnerID     string
	Name        string
	Description string
	Priority    string
	Due         time.Time
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type NewTask struct {
	ID          string
	OwnerID     string
	Name        TaskName
	Description TaskDescription
	Priority    TaskPriority
	Due         DueDate
	Status      TaskStatus
	Now         time.Time
}

type Changes struct {
	Name        *TaskName
	Description *TaskDescription
	Priority    *TaskPriority
	Due         *DueDate
	Status      *TaskStatus
}

func (c Changes) IsEmpty() bool {
	return c.Name == nil && c.Description == nil && c.Priority == nil && c.Due == nil && c.Status == nil
}

func Create(params NewTask) Task {
	timestamp := sharedomain.TruncateToMicrosecond(params.Now)
	return Task{
		id:          params.ID,
		ownerID:     params.OwnerID,
		name:        params.Name,
		description: params.Description,
		priority:    params.Priority,
		due:         params.Due,
		status:      params.Status,
		createdAt:   timestamp,
		updatedAt:   timestamp,
	}
}

func Restore(snapshot Snapshot) Task {
	return Task{
		id:          snapshot.ID,
		ownerID:     snapshot.OwnerID,
		name:        RestoreTaskName(snapshot.Name),
		description: RestoreTaskDescription(snapshot.Description),
		priority:    RestoreTaskPriority(snapshot.Priority),
		due:         RestoreDueDate(snapshot.Due),
		status:      RestoreTaskStatus(snapshot.Status),
		createdAt:   snapshot.CreatedAt.UTC(),
		updatedAt:   snapshot.UpdatedAt.UTC(),
	}
}

func (t Task) ID() string                   { return t.id }
func (t Task) OwnerID() string              { return t.ownerID }
func (t Task) Name() TaskName               { return t.name }
func (t Task) Description() TaskDescription { return t.description }
func (t Task) Priority() TaskPriority       { return t.priority }
func (t Task) Due() DueDate                 { return t.due }
func (t Task) Status() TaskStatus           { return t.status }
func (t Task) CreatedAt() time.Time         { return t.createdAt }
func (t Task) UpdatedAt() time.Time         { return t.updatedAt }

func (t Task) IsOwnedBy(userID string) bool {
	return t.ownerID == userID
}

func (t *Task) Update(changes Changes, now time.Time) {
	if changes.Name != nil {
		t.name = *changes.Name
	}
	if changes.Description != nil {
		t.description = *changes.Description
	}
	if changes.Priority != nil {
		t.priority = *changes.Priority
	}
	if changes.Due != nil {
		t.due = *changes.Due
	}
	if changes.Status != nil {
		t.status = *changes.Status
	}
	t.updatedAt = sharedomain.TruncateToMicrosecond(now)
}

func (t Task) Snapshot() Snapshot {
	return Snapshot{
		ID:          t.id,
		OwnerID:     t.ownerID,
		Name:        t.name.Value(),
		Description: t.description.Value(),
		Priority:    t.priority.Value(),
		Due:         t.due.Value(),
		Status:      t.status.Value(),
		CreatedAt:   t.createdAt,
		UpdatedAt:   t.updatedAt,
	}
}
