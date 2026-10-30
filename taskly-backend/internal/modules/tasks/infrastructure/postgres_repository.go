package infrastructure

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"taskly/backend/internal/modules/tasks/domain"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/postgres"
)

const (
	taskColumns  = "id, owner_id, name, description, priority, due, status, created_at, updated_at"
	defaultOrder = "created_at ASC, id ASC"
)

var sortColumns = map[string]string{
	"name":        "name",
	"description": "description",
	"priority":    "priority",
	"due":         "due",
	"status":      "status",
	"createdAt":   "created_at",
	"updatedAt":   "updated_at",
}

type PostgresRepository struct {
	db postgres.Querier
}

func NewPostgresRepository(db postgres.Querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *PostgresRepository) Add(ctx context.Context, task domain.Task) error {
	snapshot := task.Snapshot()
	_, err := r.db.Exec(ctx,
		"INSERT INTO tasks ("+taskColumns+") VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		snapshot.ID, snapshot.OwnerID, snapshot.Name, snapshot.Description, snapshot.Priority,
		snapshot.Due, snapshot.Status, snapshot.CreatedAt, snapshot.UpdatedAt,
	)
	return err
}

func (r *PostgresRepository) Update(ctx context.Context, task domain.Task) (bool, error) {
	snapshot := task.Snapshot()
	tag, err := r.db.Exec(ctx,
		`UPDATE tasks
		    SET name = $2,
		        description = $3,
		        priority = $4,
		        due = $5,
		        status = $6,
		        updated_at = $7
		  WHERE id = $1`,
		snapshot.ID, snapshot.Name, snapshot.Description, snapshot.Priority,
		snapshot.Due, snapshot.Status, snapshot.UpdatedAt,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (domain.Task, bool, error) {
	task, err := scanTask(r.db.QueryRow(ctx, "SELECT "+taskColumns+" FROM tasks WHERE id = $1", id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, false, nil
	}
	if err != nil {
		return domain.Task{}, false, err
	}
	return task, true, nil
}

func (r *PostgresRepository) FindByOwner(ctx context.Context, query domain.ListQuery) (domain.Page, error) {
	filterValues := []any{query.OwnerID}
	where := "owner_id = $1"
	if query.Status != "" {
		filterValues = append(filterValues, query.Status)
		where += fmt.Sprintf(" AND status = $%d", len(filterValues))
	}

	limitIndex := len(filterValues) + 1
	offsetIndex := len(filterValues) + 2
	listValues := append(append([]any{}, filterValues...), query.PageSize, (query.Page-1)*query.PageSize)

	rows, err := r.db.Query(ctx,
		fmt.Sprintf(
			"SELECT %s FROM tasks WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d",
			taskColumns, where, resolveOrderBy(query.OrderBy), limitIndex, offsetIndex,
		),
		listValues...,
	)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()

	tasks := []domain.Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return domain.Page{}, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}
	rows.Close()

	var total int
	err = r.db.QueryRow(ctx, "SELECT COUNT(*)::int FROM tasks WHERE "+where, filterValues...).Scan(&total)
	if err != nil {
		return domain.Page{}, err
	}
	return domain.Page{Tasks: tasks, Total: total}, nil
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM tasks WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) DeleteByOwner(ctx context.Context, ownerID string) (int64, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM tasks WHERE owner_id = $1", ownerID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func resolveOrderBy(orderBy string) string {
	column, ok := sortColumns[orderBy]
	if !ok {
		return defaultOrder
	}
	return column + " ASC, " + defaultOrder
}

func scanTask(row pgx.Row) (domain.Task, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.OwnerID, &snapshot.Name, &snapshot.Description, &snapshot.Priority,
		&snapshot.Due, &snapshot.Status, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.Task{}, err
	}
	return domain.Restore(snapshot), nil
}
