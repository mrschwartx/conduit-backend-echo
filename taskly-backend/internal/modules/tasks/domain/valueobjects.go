package domain

import (
	sharedomain "taskly/backend/internal/shared/domain"
)

const DefaultTaskStatus = "todo"

var (
	TaskPriorities = []string{"low", "medium", "high"}
	TaskStatuses   = []string{"todo", "in_progress", "done"}

	emptyText     = ""
	defaultStatus = DefaultTaskStatus

	taskNameRule = sharedomain.TextRule{
		Label:     "Task name",
		MinLength: 1,
		MaxLength: 120,
	}
	taskDescriptionRule = sharedomain.TextRule{
		Label:     "Task description",
		MinLength: 0,
		MaxLength: 2000,
		Default:   &emptyText,
	}
	taskPriorityRule = sharedomain.TextRule{
		Label:         "Task priority",
		MinLength:     1,
		MaxLength:     50,
		AllowedValues: TaskPriorities,
	}
	taskStatusRule = sharedomain.TextRule{
		Label:         "Task status",
		MinLength:     1,
		MaxLength:     50,
		Default:       &defaultStatus,
		AllowedValues: TaskStatuses,
	}
)

type TaskName struct{ value string }

func NewTaskName(raw sharedomain.Raw) (TaskName, error) {
	value, err := taskNameRule.Parse(raw)
	return TaskName{value: value}, err
}

func RestoreTaskName(value string) TaskName { return TaskName{value: value} }

func (v TaskName) Value() string { return v.value }

type TaskDescription struct{ value string }

func NewTaskDescription(raw sharedomain.Raw) (TaskDescription, error) {
	value, err := taskDescriptionRule.Parse(raw)
	return TaskDescription{value: value}, err
}

func RestoreTaskDescription(value string) TaskDescription { return TaskDescription{value: value} }

func (v TaskDescription) Value() string { return v.value }

type TaskPriority struct{ value string }

func NewTaskPriority(raw sharedomain.Raw) (TaskPriority, error) {
	value, err := taskPriorityRule.Parse(raw)
	return TaskPriority{value: value}, err
}

func RestoreTaskPriority(value string) TaskPriority { return TaskPriority{value: value} }

func (v TaskPriority) Value() string { return v.value }

type TaskStatus struct{ value string }

func NewTaskStatus(raw sharedomain.Raw) (TaskStatus, error) {
	value, err := taskStatusRule.Parse(raw)
	return TaskStatus{value: value}, err
}

func RestoreTaskStatus(value string) TaskStatus { return TaskStatus{value: value} }

func (v TaskStatus) Value() string { return v.value }
