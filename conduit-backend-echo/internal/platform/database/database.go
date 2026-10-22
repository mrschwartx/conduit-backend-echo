// Package database opens the MySQL pool, runs migrations and classifies errors.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate/v4"
	migmysql "github.com/golang-migrate/migrate/v4/database/mysql"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"conduit/backend/internal/platform/config"
	"conduit/backend/migrations"
)

// ErrNotFound is returned by repositories when a row does not exist.
var ErrNotFound = errors.New("not found")

// QueryTimeout bounds every repository operation.
const QueryTimeout = 5 * time.Second

// WithTimeout derives a context bounded by QueryTimeout.
func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, QueryTimeout)
}

// DSN builds the connection string. Times are parsed and stored in UTC.
func DSN(cfg config.Database, multiStatements bool) string {
	c := mysql.NewConfig()
	c.User = cfg.User
	c.Passwd = cfg.Password
	c.Net = "tcp"
	c.Addr = net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	c.DBName = cfg.Name
	c.ParseTime = true
	c.Loc = time.UTC
	c.Collation = "utf8mb4_0900_ai_ci"
	c.Timeout = 5 * time.Second
	c.MultiStatements = multiStatements
	c.Params = map[string]string{
		"charset":   "utf8mb4",
		"time_zone": "'+00:00'",
	}
	return c.FormatDSN()
}

// Open creates the pool and waits (up to 30s) until the database answers.
func Open(ctx context.Context, cfg config.Database) (*sql.DB, error) {
	db, err := sql.Open("mysql", DSN(cfg, false))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	deadline := time.Now().Add(30 * time.Second)
	for {
		pctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err = db.PingContext(pctx)
		cancel()
		if err == nil {
			return db, nil
		}
		if time.Now().After(deadline) || ctx.Err() != nil {
			_ = db.Close()
			return nil, fmt.Errorf("database not reachable: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = db.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// Migrate applies all pending versioned migrations (golang-migrate).
func Migrate(cfg config.Database) error {
	db, err := sql.Open("mysql", DSN(cfg, true))
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	drv, err := migmysql.WithInstance(db, &migmysql.Config{})
	if err != nil {
		return fmt.Errorf("migration driver: %w", err)
	}
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("migration source: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "mysql", drv)
	if err != nil {
		return fmt.Errorf("migrate init: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// IsDuplicate reports whether err is MySQL error 1062 and returns its message
// (which contains the violated index name).
func IsDuplicate(err error) (string, bool) {
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1062 {
		return me.Message, true
	}
	return "", false
}
