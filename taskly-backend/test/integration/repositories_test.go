//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	tasksdomain "taskly/backend/internal/modules/tasks/domain"
	tasksinfra "taskly/backend/internal/modules/tasks/infrastructure"
	usersdomain "taskly/backend/internal/modules/users/domain"
	usersinfra "taskly/backend/internal/modules/users/infrastructure"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/postgres"
	"taskly/backend/test/support"
)

func connect(t *testing.T) context.Context {
	t.Helper()
	return t.Context()
}

func newPool(t *testing.T) (context.Context, *usersinfra.PostgresRepository, *tasksinfra.PostgresRepository) {
	t.Helper()
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 4, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil); err != nil {
		t.Fatal(err)
	}
	return ctx, usersinfra.NewPostgresRepository(pool), tasksinfra.NewPostgresRepository(pool)
}

func buildUser(t *testing.T, repo *usersinfra.PostgresRepository, username, email string, now time.Time) usersdomain.User {
	t.Helper()
	name, err := usersdomain.NewUsername(sharedomain.RawString(username))
	if err != nil {
		t.Fatal(err)
	}
	address, err := usersdomain.NewEmail(sharedomain.RawString(email))
	if err != nil {
		t.Fatal(err)
	}
	return usersdomain.Register(usersdomain.Registration{
		ID: repo.NextID(), Username: name, Email: address, PasswordHash: "hash", Now: now,
	})
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 4, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	first, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
	if err != nil {
		t.Fatal(err)
	}
	expectEqual(t, len(first), 2)
	expectEqual(t, len(second), 0)
}

func TestMigrationsSurviveConcurrentStarts(t *testing.T) {
	ctx := connect(t)
	database := support.StartTestDatabase(t)
	pool, err := postgres.Connect(ctx, postgres.ConnectionConfig{URL: database.URL, PoolMax: 8, Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	results := make(chan error, 4)
	for range 4 {
		go func() {
			_, err := postgres.RunMigrations(ctx, pool, postgres.EmbeddedMigrations(), nil)
			results <- err
		}()
	}
	for range 4 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestUserRepository(t *testing.T) {
	ctx, users, _ := newPool(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("round trips a user", func(t *testing.T) {
		user := buildUser(t, users, "johndoe123", "John@Mail.com", base)
		if err := users.Add(ctx, user); err != nil {
			t.Fatal(err)
		}
		found, ok, err := users.FindByID(ctx, user.ID())
		if err != nil || !ok {
			t.Fatalf("find failed: %v %v", ok, err)
		}
		expectEqual(t, found.Snapshot() == user.Snapshot(), true)
	})

	t.Run("finds by email regardless of case", func(t *testing.T) {
		_, ok, err := users.FindByEmail(ctx, "john@mail.COM")
		if err != nil || !ok {
			t.Fatalf("find failed: %v %v", ok, err)
		}
	})

	t.Run("reports absence without error", func(t *testing.T) {
		_, ok, err := users.FindByID(ctx, missingID)
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, ok, false)
	})

	t.Run("rejects duplicates case insensitively", func(t *testing.T) {
		duplicate := buildUser(t, users, "JOHNDOE123", "other@mail.com", base)
		err := users.Add(ctx, duplicate)
		if !sharedomain.IsKind(err, sharedomain.KindAlreadyExists) {
			t.Fatalf("expected already exists, got %v", err)
		}
	})

	t.Run("checks uniqueness with exclusion", func(t *testing.T) {
		user, _, _ := users.FindByEmail(ctx, "john@mail.com")
		taken, err := users.ExistsByEmailOrUsername(ctx, usersdomain.UniquenessQuery{Email: "john@mail.com"})
		if err != nil || !taken {
			t.Fatalf("expected taken: %v %v", taken, err)
		}
		own, err := users.ExistsByEmailOrUsername(ctx, usersdomain.UniquenessQuery{
			Email: "john@mail.com", Username: "johndoe123", ExcludeID: user.ID(),
		})
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, own, false)
		empty, err := users.ExistsByEmailOrUsername(ctx, usersdomain.UniquenessQuery{})
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, empty, false)
	})

	t.Run("searches in creation order and escapes wildcards", func(t *testing.T) {
		second := buildUser(t, users, "jane_78", "jane@mail.com", base.Add(time.Hour))
		if err := users.Add(ctx, second); err != nil {
			t.Fatal(err)
		}
		all, err := users.Search(ctx, usersdomain.SearchFilter{})
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, len(all), 2)
		expectEqual(t, all[0].Username().Value(), "johndoe123")

		byName, _ := users.Search(ctx, usersdomain.SearchFilter{Username: "JANE"})
		expectEqual(t, len(byName), 1)
		wildcard, _ := users.Search(ctx, usersdomain.SearchFilter{Username: "%"})
		expectEqual(t, len(wildcard), 0)
		underscore, _ := users.Search(ctx, usersdomain.SearchFilter{Username: "jane_"})
		expectEqual(t, len(underscore), 1)
		other, _ := users.Search(ctx, usersdomain.SearchFilter{Username: "jane-"})
		expectEqual(t, len(other), 0)
	})

	t.Run("updates and maps unique violations", func(t *testing.T) {
		user, _, _ := users.FindByEmail(ctx, "jane@mail.com")
		clash, _ := usersdomain.NewUsername(sharedomain.RawString("johndoe123"))
		user.UpdateProfile(usersdomain.ProfileChanges{Username: &clash}, base.Add(2*time.Hour))
		if _, err := users.Update(ctx, user); !sharedomain.IsKind(err, sharedomain.KindAlreadyExists) {
			t.Fatalf("expected already exists, got %v", err)
		}

		free, _ := usersdomain.NewUsername(sharedomain.RawString("jane_renamed"))
		user.UpdateProfile(usersdomain.ProfileChanges{Username: &free}, base.Add(2*time.Hour))
		updated, err := users.Update(ctx, user)
		if err != nil || !updated {
			t.Fatalf("update failed: %v %v", updated, err)
		}
	})

	t.Run("deletes users", func(t *testing.T) {
		user, _, _ := users.FindByEmail(ctx, "jane@mail.com")
		deleted, err := users.DeleteByID(ctx, user.ID())
		if err != nil || !deleted {
			t.Fatalf("delete failed: %v %v", deleted, err)
		}
		again, _ := users.DeleteByID(ctx, user.ID())
		expectEqual(t, again, false)
	})
}

func TestTaskRepository(t *testing.T) {
	ctx, users, tasks := newPool(t)
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	owner := buildUser(t, users, "owner_one", "owner@mail.com", base)
	other := buildUser(t, users, "owner_two", "two@mail.com", base)
	for _, user := range []usersdomain.User{owner, other} {
		if err := users.Add(ctx, user); err != nil {
			t.Fatal(err)
		}
	}

	build := func(ownerID, name, status string, offset time.Duration) tasksdomain.Task {
		taskName, _ := tasksdomain.NewTaskName(sharedomain.RawString(name))
		description, _ := tasksdomain.NewTaskDescription(sharedomain.Raw{})
		priority, _ := tasksdomain.NewTaskPriority(sharedomain.RawString("low"))
		due, _ := tasksdomain.NewDueDate(sharedomain.RawString("2026-12-08T10:00:00Z"))
		taskStatus, _ := tasksdomain.NewTaskStatus(sharedomain.RawString(status))
		return tasksdomain.Create(tasksdomain.NewTask{
			ID: tasks.NextID(), OwnerID: ownerID, Name: taskName, Description: description,
			Priority: priority, Due: due, Status: taskStatus, Now: base.Add(offset),
		})
	}

	var first tasksdomain.Task
	for index, name := range []string{"d", "b", "a", "c", "e"} {
		status := "todo"
		if index%2 == 1 {
			status = "done"
		}
		task := build(owner.ID(), name, status, time.Duration(index)*time.Minute)
		if index == 0 {
			first = task
		}
		if err := tasks.Add(ctx, task); err != nil {
			t.Fatal(err)
		}
	}
	if err := tasks.Add(ctx, build(other.ID(), "foreign", "todo", 0)); err != nil {
		t.Fatal(err)
	}

	names := func(page tasksdomain.Page) string {
		result := ""
		for _, task := range page.Tasks {
			result += task.Name().Value()
		}
		return result
	}

	t.Run("round trips a task", func(t *testing.T) {
		found, ok, err := tasks.FindByID(ctx, first.ID())
		if err != nil || !ok {
			t.Fatalf("find failed: %v %v", ok, err)
		}
		expectEqual(t, found.Snapshot() == first.Snapshot(), true)
	})

	t.Run("paginates in creation order", func(t *testing.T) {
		page1, err := tasks.FindByOwner(ctx, tasksdomain.ListQuery{OwnerID: owner.ID(), Page: 1, PageSize: 4})
		if err != nil {
			t.Fatal(err)
		}
		page2, _ := tasks.FindByOwner(ctx, tasksdomain.ListQuery{OwnerID: owner.ID(), Page: 2, PageSize: 4})
		page3, _ := tasks.FindByOwner(ctx, tasksdomain.ListQuery{OwnerID: owner.ID(), Page: 3, PageSize: 4})
		expectEqual(t, names(page1), "dbac")
		expectEqual(t, names(page2), "e")
		expectEqual(t, names(page3), "")
		expectEqual(t, page1.Total, 5)
	})

	t.Run("filters and orders by a whitelisted column", func(t *testing.T) {
		page, err := tasks.FindByOwner(ctx, tasksdomain.ListQuery{
			OwnerID: owner.ID(), Status: "todo", OrderBy: "name", Page: 1, PageSize: 4,
		})
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, names(page), "ade")
		expectEqual(t, page.Total, 3)
	})

	t.Run("ignores unknown order columns", func(t *testing.T) {
		page, err := tasks.FindByOwner(ctx, tasksdomain.ListQuery{
			OwnerID: owner.ID(), OrderBy: "name; DROP TABLE tasks", Page: 1, PageSize: 4,
		})
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, names(page), "dbac")
	})

	t.Run("updates tasks", func(t *testing.T) {
		renamed, _ := tasksdomain.NewTaskName(sharedomain.RawString("renamed"))
		first.Update(tasksdomain.Changes{Name: &renamed}, base.Add(time.Hour))
		updated, err := tasks.Update(ctx, first)
		if err != nil || !updated {
			t.Fatalf("update failed: %v %v", updated, err)
		}
		found, _, _ := tasks.FindByID(ctx, first.ID())
		expectEqual(t, found.Name().Value(), "renamed")
	})

	t.Run("deletes by id and by owner", func(t *testing.T) {
		deleted, err := tasks.DeleteByID(ctx, first.ID())
		if err != nil || !deleted {
			t.Fatalf("delete failed: %v %v", deleted, err)
		}
		count, err := tasks.DeleteByOwner(ctx, owner.ID())
		if err != nil {
			t.Fatal(err)
		}
		expectEqual(t, count, 4)
		remaining, _ := tasks.FindByOwner(ctx, tasksdomain.ListQuery{OwnerID: other.ID(), Page: 1, PageSize: 4})
		expectEqual(t, remaining.Total, 1)
	})

	t.Run("cascades when the owner is deleted", func(t *testing.T) {
		if _, err := users.DeleteByID(ctx, other.ID()); err != nil {
			t.Fatal(err)
		}
		remaining, _ := tasks.FindByOwner(ctx, tasksdomain.ListQuery{OwnerID: other.ID(), Page: 1, PageSize: 4})
		expectEqual(t, remaining.Total, 0)
	})
}
