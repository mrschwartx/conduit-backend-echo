package application

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
)

const (
	ownerID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	otherID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

type fakeRepository struct {
	tasks  map[string]domain.Task
	nextID int
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{tasks: map[string]domain.Task{}}
}

func (r *fakeRepository) NextID() string {
	r.nextID++
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", r.nextID)
}

func (r *fakeRepository) Add(_ context.Context, task domain.Task) error {
	r.tasks[task.ID()] = task
	return nil
}

func (r *fakeRepository) Update(_ context.Context, task domain.Task) (bool, error) {
	if _, ok := r.tasks[task.ID()]; !ok {
		return false, nil
	}
	r.tasks[task.ID()] = task
	return true, nil
}

func (r *fakeRepository) FindByID(_ context.Context, id string) (domain.Task, bool, error) {
	task, ok := r.tasks[id]
	return task, ok, nil
}

func (r *fakeRepository) FindByOwner(_ context.Context, query domain.ListQuery) (domain.Page, error) {
	matching := []domain.Task{}
	for _, task := range r.tasks {
		if task.OwnerID() == query.OwnerID && (query.Status == "" || task.Status().Value() == query.Status) {
			matching = append(matching, task)
		}
	}
	sort.Slice(matching, func(i, j int) bool { return matching[i].ID() < matching[j].ID() })
	start := min((query.Page-1)*query.PageSize, len(matching))
	end := min(start+query.PageSize, len(matching))
	return domain.Page{Tasks: matching[start:end], Total: len(matching)}, nil
}

func (r *fakeRepository) DeleteByID(_ context.Context, id string) (bool, error) {
	if _, ok := r.tasks[id]; !ok {
		return false, nil
	}
	delete(r.tasks, id)
	return true, nil
}

func (r *fakeRepository) DeleteByOwner(_ context.Context, owner string) (int64, error) {
	var count int64
	for id, task := range r.tasks {
		if task.OwnerID() == owner {
			delete(r.tasks, id)
			count++
		}
	}
	return count, nil
}

func validInput() TaskInput {
	return TaskInput{
		Name:        sharedomain.RawString("Read Atomic Habits"),
		Description: sharedomain.RawString("Finish the book"),
		Priority:    sharedomain.RawString("low"),
		Due:         sharedomain.RawString("2026-12-08T10:00:00Z"),
		Status:      sharedomain.RawString("todo"),
	}
}

func expectKind(t *testing.T, err error, kind sharedomain.ErrorKind, message string) {
	t.Helper()
	if !sharedomain.IsKind(err, kind) {
		t.Fatalf("expected kind %s, got %v", kind, err)
	}
	if message != "" && err.Error() != message {
		t.Fatalf("message = %q, want %q", err.Error(), message)
	}
}

func seed(t *testing.T, repo *fakeRepository, owner string) TaskDTO {
	t.Helper()
	create := NewCreateTask(repo)
	create.now = func() time.Time { return time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC) }
	task, err := create.Execute(context.Background(), owner, validInput())
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestCreateTask(t *testing.T) {
	repo := newFakeRepository()
	task := seed(t, repo, ownerID)

	if task.Owner != ownerID || task.Status != "todo" || task.Due != "2026-12-08T10:00:00.000Z" {
		t.Fatalf("unexpected task %+v", task)
	}
	if task.CreatedAt != "2026-05-01T12:00:00.000Z" || task.CreatedAt != task.UpdatedAt {
		t.Fatalf("unexpected timestamps %+v", task)
	}

	create := NewCreateTask(repo)
	input := validInput()
	input.Status = sharedomain.Raw{}
	input.Description = sharedomain.Raw{}
	defaulted, err := create.Execute(context.Background(), ownerID, input)
	if err != nil || defaulted.Status != "todo" || defaulted.Description != "" {
		t.Fatalf("defaults failed: %v %+v", err, defaulted)
	}

	input.Priority = sharedomain.RawString("urgent")
	_, err = create.Execute(context.Background(), ownerID, input)
	expectKind(t, err, sharedomain.KindValidation, "Task priority must be one of: low, medium, high")
	if len(repo.tasks) != 2 {
		t.Fatal("invalid tasks must not be stored")
	}
}

func TestGetTaskEnforcesOwnership(t *testing.T) {
	repo := newFakeRepository()
	task := seed(t, repo, ownerID)
	get := NewGetTask(repo)
	ctx := context.Background()

	found, err := get.Execute(ctx, TaskAccess{RequesterID: ownerID, TaskID: task.ID})
	if err != nil || found.ID != task.ID {
		t.Fatalf("get failed: %v", err)
	}
	_, err = get.Execute(ctx, TaskAccess{RequesterID: otherID, TaskID: task.ID})
	expectKind(t, err, sharedomain.KindForbidden, "You are not authorized to view this task")
	_, err = get.Execute(ctx, TaskAccess{RequesterID: ownerID, TaskID: otherID})
	expectKind(t, err, sharedomain.KindNotFound, "Task not found!")
	_, err = get.Execute(ctx, TaskAccess{RequesterID: ownerID, TaskID: "bad"})
	expectKind(t, err, sharedomain.KindValidation, "Invalid task ID format")
}

func TestListTasksByUser(t *testing.T) {
	repo := newFakeRepository()
	for range 6 {
		seed(t, repo, ownerID)
	}
	seed(t, repo, otherID)
	list := NewListTasksByUser(repo)
	ctx := context.Background()

	first, err := list.Execute(ctx, ListTasksInput{RequesterID: ownerID, OwnerID: ownerID})
	if err != nil || len(first.Tasks) != TasksPageSize || first.TaskCount != 6 {
		t.Fatalf("page one failed: %v %+v", err, first)
	}
	second, _ := list.Execute(ctx, ListTasksInput{RequesterID: ownerID, OwnerID: ownerID, Page: []string{"2"}})
	if len(second.Tasks) != 2 {
		t.Fatalf("page two has %d tasks", len(second.Tasks))
	}

	_, err = list.Execute(ctx, ListTasksInput{RequesterID: otherID, OwnerID: ownerID})
	expectKind(t, err, sharedomain.KindForbidden, "You are not authorized to view these tasks")
	_, err = list.Execute(ctx, ListTasksInput{RequesterID: ownerID, OwnerID: ownerID, Status: []string{"a", "b"}})
	expectKind(t, err, sharedomain.KindValidation, "Status filter must be a string")
	_, err = list.Execute(ctx, ListTasksInput{RequesterID: ownerID, OwnerID: ownerID, OrderBy: []string{"owner"}})
	expectKind(t, err, sharedomain.KindValidation, "Invalid orderBy field")
	_, err = list.Execute(ctx, ListTasksInput{RequesterID: ownerID, OwnerID: "nope"})
	expectKind(t, err, sharedomain.KindValidation, "Invalid user ID format")

	empty, err := list.Execute(ctx, ListTasksInput{RequesterID: otherID, OwnerID: otherID, Status: []string{"done"}})
	if err != nil || empty.Tasks == nil || len(empty.Tasks) != 0 || empty.TaskCount != 0 {
		t.Fatalf("empty list must be an empty slice: %v %+v", err, empty)
	}
}

func TestNormalizePage(t *testing.T) {
	cases := map[string]int{"3": 3, "0": 1, "-4": 1, "abc": 1, "": 1, "2abc": 2, "99999999999999999999": 1, "5000000": maxPage}
	for input, want := range cases {
		if got := normalizePage([]string{input}); got != want {
			t.Errorf("normalizePage(%q) = %d, want %d", input, got, want)
		}
	}
	if normalizePage(nil) != 1 {
		t.Error("missing page defaults to 1")
	}
}

func TestUpdateTask(t *testing.T) {
	repo := newFakeRepository()
	task := seed(t, repo, ownerID)
	update := NewUpdateTask(repo)
	update.now = func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }
	ctx := context.Background()
	access := TaskAccess{RequesterID: ownerID, TaskID: task.ID}

	updated, err := update.Execute(ctx, UpdateTaskInput{Access: access, Changes: TaskInput{
		Name: sharedomain.RawString("Renamed"), Status: sharedomain.RawString("done"),
	}})
	if err != nil || updated.Name != "Renamed" || updated.Status != "done" || updated.Priority != "low" {
		t.Fatalf("update failed: %v %+v", err, updated)
	}
	if updated.UpdatedAt != "2026-06-01T00:00:00.000Z" || updated.CreatedAt != task.CreatedAt {
		t.Fatalf("unexpected timestamps %+v", updated)
	}

	_, err = update.Execute(ctx, UpdateTaskInput{Access: access})
	expectKind(t, err, sharedomain.KindValidation, "No updatable fields provided")
	_, err = update.Execute(ctx, UpdateTaskInput{Access: access, Changes: TaskInput{Due: sharedomain.RawString("someday")}})
	expectKind(t, err, sharedomain.KindValidation, "Task due date is invalid")
	_, err = update.Execute(ctx, UpdateTaskInput{
		Access:  TaskAccess{RequesterID: otherID, TaskID: task.ID},
		Changes: TaskInput{Name: sharedomain.RawString("Hacked")},
	})
	expectKind(t, err, sharedomain.KindForbidden, "You are not authorized to update this task")
	if repo.tasks[task.ID].Name().Value() != "Renamed" {
		t.Fatal("forbidden update must not change the task")
	}
}

func TestDeleteTaskAndOwnerCleanup(t *testing.T) {
	repo := newFakeRepository()
	mine := seed(t, repo, ownerID)
	seed(t, repo, ownerID)
	theirs := seed(t, repo, otherID)
	ctx := context.Background()

	err := NewDeleteTask(repo).Execute(ctx, TaskAccess{RequesterID: otherID, TaskID: mine.ID})
	expectKind(t, err, sharedomain.KindForbidden, "You are not authorized to delete this task")
	if err := NewDeleteTask(repo).Execute(ctx, TaskAccess{RequesterID: ownerID, TaskID: mine.ID}); err != nil {
		t.Fatal(err)
	}
	err = NewDeleteTask(repo).Execute(ctx, TaskAccess{RequesterID: ownerID, TaskID: mine.ID})
	expectKind(t, err, sharedomain.KindNotFound, "Task not found!")

	removed, err := NewDeleteTasksByOwner(repo).Execute(ctx, ownerID)
	if err != nil || removed != 1 {
		t.Fatalf("cleanup failed: %v %d", err, removed)
	}
	if _, ok := repo.tasks[theirs.ID]; !ok {
		t.Fatal("other users' tasks must survive")
	}
	_, err = NewDeleteTasksByOwner(repo).Execute(ctx, "bad")
	expectKind(t, err, sharedomain.KindValidation, "Invalid user ID format")
}
