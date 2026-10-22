// Package config loads the application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Database holds the MySQL connection settings.
type Database struct {
	Host            string
	Port            int
	Name            string
	User            string
	Password        string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// Config is the full application configuration.
type Config struct {
	Port               string
	DB                 Database
	RunMigrations      bool
	JWTSecret          string
	JWTTTL             time.Duration
	CORSAllowedOrigins []string
	LogLevel           string
	BcryptCost         int
}

// Load reads the environment. It fails fast when JWT_SECRET is missing or short.
func Load() (Config, error) {
	var errs []error

	getenv := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}
	getInt := func(key string, def int) int {
		raw := os.Getenv(key)
		if raw == "" {
			return def
		}
		n, err := strconv.Atoi(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s must be an integer", key))
			return def
		}
		return n
	}

	cfg := Config{
		Port: getenv("PORT", "8080"),
		DB: Database{
			Host: getenv("DATABASE_HOST", "localhost"),
			Port: getInt("DATABASE_PORT", 3306),
			Name: getenv("DATABASE_NAME", "conduit"),
			User: getenv("DATABASE_USER", "conduit"),
			// Dev default, identical to conduit-docker/docker-compose.local.yml.
			Password:        getenv("DATABASE_PASSWORD", "S9p!tBox_2026@Db"),
			MaxOpenConns:    getInt("DATABASE_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getInt("DATABASE_MAX_IDLE_CONNS", 10),
			ConnMaxLifetime: time.Duration(getInt("DATABASE_CONN_MAX_LIFETIME_MINUTES", 30)) * time.Minute,
		},
		RunMigrations:      strings.ToLower(getenv("DATABASE_RUN_MIGRATIONS", "true")) != "false",
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTTTL:             time.Duration(getInt("JWT_TTL_MINUTES", 720)) * time.Minute,
		CORSAllowedOrigins: splitCSV(getenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		LogLevel:           getenv("LOG_LEVEL", "info"),
		BcryptCost:         getInt("BCRYPT_COST", 12),
	}

	if len(cfg.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET is required and must be at least 32 characters"))
	}
	if cfg.JWTTTL <= 0 {
		errs = append(errs, errors.New("JWT_TTL_MINUTES must be greater than 0"))
	}
	if cfg.BcryptCost < 4 || cfg.BcryptCost > 31 {
		errs = append(errs, errors.New("BCRYPT_COST must be between 4 and 31"))
	}
	return cfg, errors.Join(errs...)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
