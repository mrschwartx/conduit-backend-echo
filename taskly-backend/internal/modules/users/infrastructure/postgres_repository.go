package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"taskly/backend/internal/modules/users/domain"
	sharedomain "taskly/backend/internal/shared/domain"
	"taskly/backend/internal/shared/infrastructure/postgres"
)

const (
	duplicateMessage = "Email or Username is already registered."
	userColumns      = "id, username, email, password_hash, avatar_file, created_at, updated_at"
)

type PostgresRepository struct {
	db postgres.Querier
}

func NewPostgresRepository(db postgres.Querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) NextID() string {
	return sharedomain.NewEntityID()
}

func (r *PostgresRepository) Add(ctx context.Context, user domain.User) error {
	snapshot := user.Snapshot()
	_, err := r.db.Exec(ctx,
		"INSERT INTO users ("+userColumns+") VALUES ($1, $2, $3, $4, $5, $6, $7)",
		snapshot.ID, snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Avatar, snapshot.CreatedAt, snapshot.UpdatedAt,
	)
	return translateUniqueViolation(err)
}

func (r *PostgresRepository) Update(ctx context.Context, user domain.User) (bool, error) {
	snapshot := user.Snapshot()
	tag, err := r.db.Exec(ctx,
		`UPDATE users
		    SET username = $2,
		        email = $3,
		        password_hash = $4,
		        avatar_file = $5,
		        updated_at = $6
		  WHERE id = $1`,
		snapshot.ID, snapshot.Username, snapshot.Email, snapshot.PasswordHash,
		snapshot.Avatar, snapshot.UpdatedAt,
	)
	if err != nil {
		return false, translateUniqueViolation(err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) FindByID(ctx context.Context, id string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id)
}

func (r *PostgresRepository) FindByEmail(ctx context.Context, email string) (domain.User, bool, error) {
	return r.findOne(ctx, "SELECT "+userColumns+" FROM users WHERE lower(email) = lower($1)", email)
}

func (r *PostgresRepository) ExistsByEmailOrUsername(ctx context.Context, query domain.UniquenessQuery) (bool, error) {
	values := []any{}
	conditions := []string{}
	if query.Email != "" {
		values = append(values, query.Email)
		conditions = append(conditions, fmt.Sprintf("lower(email) = lower($%d)", len(values)))
	}
	if query.Username != "" {
		values = append(values, query.Username)
		conditions = append(conditions, fmt.Sprintf("lower(username) = lower($%d)", len(values)))
	}
	if len(conditions) == 0 {
		return false, nil
	}

	sql := "SELECT 1 FROM users WHERE (" + strings.Join(conditions, " OR ") + ")"
	if query.ExcludeID != "" {
		values = append(values, query.ExcludeID)
		sql += fmt.Sprintf(" AND id <> $%d", len(values))
	}

	var marker int
	err := r.db.QueryRow(ctx, sql+" LIMIT 1", values...).Scan(&marker)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (r *PostgresRepository) Search(ctx context.Context, filter domain.SearchFilter) ([]domain.User, error) {
	values := []any{}
	conditions := []string{}
	if filter.Username != "" {
		values = append(values, "%"+escapeLikePattern(filter.Username)+"%")
		conditions = append(conditions, fmt.Sprintf("username ILIKE $%d", len(values)))
	}
	if filter.Email != "" {
		values = append(values, "%"+escapeLikePattern(filter.Email)+"%")
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", len(values)))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	rows, err := r.db.Query(ctx,
		"SELECT "+userColumns+" FROM users "+where+" ORDER BY created_at ASC, id ASC",
		values...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *PostgresRepository) DeleteByID(ctx context.Context, id string) (bool, error) {
	tag, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (r *PostgresRepository) findOne(ctx context.Context, sql string, args ...any) (domain.User, bool, error) {
	user, err := scanUser(r.db.QueryRow(ctx, sql, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, false, nil
	}
	if err != nil {
		return domain.User{}, false, err
	}
	return user, true, nil
}

func scanUser(row pgx.Row) (domain.User, error) {
	var snapshot domain.Snapshot
	err := row.Scan(
		&snapshot.ID, &snapshot.Username, &snapshot.Email, &snapshot.PasswordHash,
		&snapshot.Avatar, &snapshot.CreatedAt, &snapshot.UpdatedAt,
	)
	if err != nil {
		return domain.User{}, err
	}
	return domain.Restore(snapshot), nil
}

func translateUniqueViolation(err error) error {
	if postgres.IsUniqueViolation(err) {
		return sharedomain.NewAlreadyExistsError(duplicateMessage)
	}
	return err
}

func escapeLikePattern(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}
