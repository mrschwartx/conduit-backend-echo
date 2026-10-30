//go:build integration

package integration

import (
	"fmt"
	"testing"

	"taskly/backend/internal/modules/tasks/application"
	"taskly/backend/test/support"
)

func createTask(t *testing.T, user signedIn, overrides map[string]any) support.Response {
	t.Helper()
	response := user.client.Post("/api/v1/tasks/create", support.RequestOptions{JSON: support.TaskPayload(overrides)})
	expectStatus(t, response, 200)
	return response
}

func TestTasksRequireAuthentication(t *testing.T) {
	app := startApp(t)
	client := app.NewClient()
	responses := []support.Response{
		client.Get("/api/v1/tasks/" + missingID),
		client.Get("/api/v1/tasks/user/" + missingID),
		client.Post("/api/v1/tasks/create", support.RequestOptions{JSON: support.TaskPayload(nil)}),
		client.Patch("/api/v1/tasks/"+missingID, support.RequestOptions{JSON: map[string]any{}}),
		client.Delete("/api/v1/tasks/" + missingID),
	}
	for _, response := range responses {
		expectStatus(t, response, 401)
	}
}

func TestCreateTask(t *testing.T) {
	app := startApp(t)

	t.Run("creates a task owned by the authenticated user", func(t *testing.T) {
		user := signIn(t, app)
		response := createTask(t, user, nil)

		if !uuidPattern.MatchString(response.String("_id")) {
			t.Fatalf("unexpected id %q", response.String("_id"))
		}
		expectEqual(t, response.String("owner"), user.id)
		expectEqual(t, response.String("name"), "Read Atomic Habits")
		expectEqual(t, response.String("priority"), "low")
		expectEqual(t, response.String("status"), "todo")
		expectEqual(t, response.String("due"), "2026-12-08T10:00:00.000Z")
		expectEqual(t, response.String("createdAt"), response.String("updatedAt"))
	})

	t.Run("defaults the status and ignores a spoofed owner", func(t *testing.T) {
		user := signIn(t, app)
		other := signIn(t, app)
		response := createTask(t, user, map[string]any{"status": nil, "owner": other.id})
		expectEqual(t, response.String("status"), "todo")
		expectEqual(t, response.String("owner"), user.id)
	})

	t.Run("rejects invalid payloads", func(t *testing.T) {
		user := signIn(t, app)
		payloads := []map[string]any{
			{},
			support.TaskPayload(map[string]any{"name": ""}),
			support.TaskPayload(map[string]any{"due": "someday"}),
			support.TaskPayload(map[string]any{"priority": nil}),
			support.TaskPayload(map[string]any{"description": nil}),
			support.TaskPayload(map[string]any{"name": 42}),
		}
		for index, payload := range payloads {
			if index == 4 {
				payload["description"] = nil
			}
			response := user.client.Post("/api/v1/tasks/create", support.RequestOptions{JSON: payload})
			expectStatus(t, response, 400)
		}
	})
}

func TestGetTask(t *testing.T) {
	app := startApp(t)

	t.Run("returns the task to its owner", func(t *testing.T) {
		user := signIn(t, app)
		created := createTask(t, user, nil)
		response := user.client.Get("/api/v1/tasks/" + created.String("_id"))
		expectStatus(t, response, 200)
		expectEqual(t, string(response.Raw), string(created.Raw))
	})

	t.Run("answers 403 for tasks of other users", func(t *testing.T) {
		owner := signIn(t, app)
		intruder := signIn(t, app)
		created := createTask(t, owner, nil)
		expectStatus(t, intruder.client.Get("/api/v1/tasks/"+created.String("_id")), 403)
	})

	t.Run("answers 404 for unknown ids and 400 for malformed ids", func(t *testing.T) {
		user := signIn(t, app)
		unknown := user.client.Get("/api/v1/tasks/" + missingID)
		expectStatus(t, unknown, 404)
		expectMessage(t, unknown, "Task not found!")
		expectStatus(t, user.client.Get("/api/v1/tasks/nope"), 400)
	})
}

func TestListTasks(t *testing.T) {
	app := startApp(t)

	taskNames := func(response support.Response) []string {
		names := []string{}
		for _, item := range response.Body["tasks"].([]any) {
			names = append(names, item.(map[string]any)["name"].(string))
		}
		return names
	}

	t.Run("paginates four tasks per page and reports the total", func(t *testing.T) {
		user := signIn(t, app)
		for index := 1; index <= 6; index++ {
			createTask(t, user, map[string]any{"name": fmt.Sprintf("Task %d", index)})
		}

		first := user.client.Get("/api/v1/tasks/user/" + user.id)
		second := user.client.Get("/api/v1/tasks/user/" + user.id + "?page=2")

		expectStatus(t, first, 200)
		expectEqual(t, first.Body["taskCount"].(float64), 6)
		expectEqual(t, len(taskNames(first)), application.TasksPageSize)
		expectEqual(t, len(taskNames(second)), 2)
		all := append(taskNames(first), taskNames(second)...)
		expectEqual(t, fmt.Sprint(all), "[Task 1 Task 2 Task 3 Task 4 Task 5 Task 6]")
	})

	t.Run("falls back to the first page for invalid page numbers", func(t *testing.T) {
		user := signIn(t, app)
		createTask(t, user, map[string]any{"name": "Only"})
		for _, page := range []string{"abc", "0", "-3", ""} {
			response := user.client.Get("/api/v1/tasks/user/" + user.id + "?page=" + page)
			expectEqual(t, fmt.Sprint(taskNames(response)), "[Only]")
		}
	})

	t.Run("filters by status and orders by a field", func(t *testing.T) {
		user := signIn(t, app)
		createTask(t, user, map[string]any{"name": "b-task", "status": "done"})
		createTask(t, user, map[string]any{"name": "a-task", "status": "done"})
		createTask(t, user, map[string]any{"name": "c-task", "status": "todo"})

		response := user.client.Get("/api/v1/tasks/user/" + user.id + "?status=done&orderBy=name")
		expectEqual(t, response.Body["taskCount"].(float64), 2)
		expectEqual(t, fmt.Sprint(taskNames(response)), "[a-task b-task]")
	})

	t.Run("returns an empty page for a user without tasks", func(t *testing.T) {
		user := signIn(t, app)
		response := user.client.Get("/api/v1/tasks/user/" + user.id)
		expectEqual(t, string(response.Raw), "{\"tasks\":[],\"taskCount\":0}\n")
	})

	t.Run("forbids reading the tasks of another user", func(t *testing.T) {
		owner := signIn(t, app)
		intruder := signIn(t, app)
		expectStatus(t, intruder.client.Get("/api/v1/tasks/user/"+owner.id), 403)
	})

	t.Run("rejects invalid query parameters", func(t *testing.T) {
		user := signIn(t, app)
		expectStatus(t, user.client.Get("/api/v1/tasks/user/"+user.id+"?orderBy=owner"), 400)
		expectStatus(t, user.client.Get("/api/v1/tasks/user/"+user.id+"?orderBy="), 400)
		expectStatus(t, user.client.Get("/api/v1/tasks/user/"+user.id+"?status=a&status=b"), 400)
		expectStatus(t, user.client.Get("/api/v1/tasks/user/not-an-id"), 400)
	})
}

func TestUpdateTask(t *testing.T) {
	app := startApp(t)

	t.Run("updates the task", func(t *testing.T) {
		user := signIn(t, app)
		created := createTask(t, user, nil)
		response := user.client.Patch("/api/v1/tasks/"+created.String("_id"), support.RequestOptions{JSON: map[string]any{
			"name": "Read Atomic Habits - Updated", "priority": "high", "status": "in_progress", "due": "2026-12-15T10:00:00Z",
		}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("name"), "Read Atomic Habits - Updated")
		expectEqual(t, response.String("status"), "in_progress")
		expectEqual(t, response.String("due"), "2026-12-15T10:00:00.000Z")
		expectEqual(t, response.String("owner"), created.String("owner"))
		expectEqual(t, response.String("createdAt"), created.String("createdAt"))
		if response.String("updatedAt") < created.String("updatedAt") {
			t.Fatal("updatedAt moved backwards")
		}
	})

	t.Run("cannot transfer a task to another user", func(t *testing.T) {
		owner := signIn(t, app)
		other := signIn(t, app)
		created := createTask(t, owner, nil)
		response := owner.client.Patch("/api/v1/tasks/"+created.String("_id"), support.RequestOptions{JSON: map[string]any{
			"name": "Still mine", "owner": other.id,
		}})
		expectStatus(t, response, 200)
		expectEqual(t, response.String("owner"), owner.id)
	})

	t.Run("answers 403 for tasks of other users and leaves them untouched", func(t *testing.T) {
		owner := signIn(t, app)
		intruder := signIn(t, app)
		created := createTask(t, owner, nil)
		response := intruder.client.Patch("/api/v1/tasks/"+created.String("_id"), support.RequestOptions{JSON: map[string]any{"name": "Hacked"}})
		unchanged := owner.client.Get("/api/v1/tasks/" + created.String("_id"))
		expectStatus(t, response, 403)
		expectEqual(t, unchanged.String("name"), "Read Atomic Habits")
	})

	t.Run("rejects empty and invalid updates", func(t *testing.T) {
		user := signIn(t, app)
		created := createTask(t, user, nil)
		path := "/api/v1/tasks/" + created.String("_id")
		expectStatus(t, user.client.Patch(path, support.RequestOptions{JSON: map[string]any{}}), 400)
		expectStatus(t, user.client.Patch(path, support.RequestOptions{JSON: map[string]any{"owner": "x"}}), 400)
		expectStatus(t, user.client.Patch(path, support.RequestOptions{JSON: map[string]any{"due": "never"}}), 400)
		expectStatus(t, user.client.Patch(path, support.RequestOptions{JSON: map[string]any{"name": ""}}), 400)
	})
}

func TestDeleteTask(t *testing.T) {
	app := startApp(t)

	t.Run("deletes the task", func(t *testing.T) {
		user := signIn(t, app)
		created := createTask(t, user, nil)
		response := user.client.Delete("/api/v1/tasks/" + created.String("_id"))
		expectStatus(t, response, 200)
		expectMessage(t, response, "Task has been deleted")
		expectStatus(t, user.client.Get("/api/v1/tasks/"+created.String("_id")), 404)
	})

	t.Run("answers 403 for tasks of other users and keeps them", func(t *testing.T) {
		owner := signIn(t, app)
		intruder := signIn(t, app)
		created := createTask(t, owner, nil)
		expectStatus(t, intruder.client.Delete("/api/v1/tasks/"+created.String("_id")), 403)
		expectStatus(t, owner.client.Get("/api/v1/tasks/"+created.String("_id")), 200)
	})

	t.Run("answers 404 for unknown tasks", func(t *testing.T) {
		user := signIn(t, app)
		expectStatus(t, user.client.Delete("/api/v1/tasks/"+missingID), 404)
	})
}

func TestUserDeletionRemovesTasks(t *testing.T) {
	app := startApp(t)
	leaving := signIn(t, app)
	staying := signIn(t, app)
	createTask(t, leaving, nil)
	createTask(t, leaving, nil)
	createTask(t, staying, nil)

	expectStatus(t, leaving.client.Delete("/api/v1/users/delete/"+leaving.id), 200)

	gone, err := app.Container.Tasks.UseCases.ListTasksByUser.Execute(t.Context(), application.ListTasksInput{
		RequesterID: leaving.id, OwnerID: leaving.id,
	})
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, gone.TaskCount, 0)

	kept := staying.client.Get("/api/v1/tasks/user/" + staying.id)
	expectEqual(t, kept.Body["taskCount"].(float64), 1)
}
