package application

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	TasksPageSize = 4
	maxPage       = 1_000_000
)

var (
	sortableFields = []string{"name", "description", "priority", "due", "status", "createdAt", "updatedAt"}
	leadingInteger = regexp.MustCompile(`^[+-]?\d+`)
)

type ListTasksInput struct {
	RequesterID string
	OwnerID     string
	Status      []string
	OrderBy     []string
	Page        []string
}

type ListTasksByUser struct {
	tasks domain.Repository
}

func NewListTasksByUser(tasks domain.Repository) *ListTasksByUser {
	return &ListTasksByUser{tasks: tasks}
}

func (uc *ListTasksByUser) Execute(ctx context.Context, input ListTasksInput) (TaskListDTO, error) {
	owner, err := sharedomain.ParseEntityID(input.OwnerID, "user")
	if err != nil {
		return TaskListDTO{}, err
	}
	if owner != input.RequesterID {
		return TaskListDTO{}, sharedomain.NewForbiddenError("You are not authorized to view these tasks")
	}
	if len(input.Status) > 1 {
		return TaskListDTO{}, sharedomain.NewValidationError("Status filter must be a string")
	}
	orderBy, err := parseOrderBy(input.OrderBy)
	if err != nil {
		return TaskListDTO{}, err
	}

	status := ""
	if len(input.Status) == 1 {
		status = input.Status[0]
	}

	page, err := uc.tasks.FindByOwner(ctx, domain.ListQuery{
		OwnerID:  owner,
		Status:   status,
		OrderBy:  orderBy,
		Page:     normalizePage(input.Page),
		PageSize: TasksPageSize,
	})
	if err != nil {
		return TaskListDTO{}, err
	}

	result := TaskListDTO{Tasks: make([]TaskDTO, 0, len(page.Tasks)), TaskCount: page.Total}
	for _, task := range page.Tasks {
		result.Tasks = append(result.Tasks, ToTaskDTO(task))
	}
	return result, nil
}

func parseOrderBy(values []string) (string, error) {
	if len(values) == 0 {
		return "", nil
	}
	if len(values) > 1 || !slices.Contains(sortableFields, values[0]) {
		return "", sharedomain.NewValidationError("Invalid orderBy field")
	}
	return values[0], nil
}

func normalizePage(values []string) int {
	if len(values) == 0 {
		return 1
	}
	match := leadingInteger.FindString(strings.TrimSpace(values[0]))
	if match == "" {
		return 1
	}
	parsed, err := strconv.Atoi(match)
	if err != nil || parsed < 1 {
		return 1
	}
	return min(parsed, maxPage)
}
