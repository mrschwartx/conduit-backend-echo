package support

import (
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"taskly/backend/internal/config"
)

const TestAuthSecret = "test-secret-value-with-more-than-32-chars"

func ProjectRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestEnv(overrides map[string]string) map[string]string {
	env := map[string]string{
		"AUTH_SECRET":   TestAuthSecret,
		"DATABASE_URL":  "postgres://127.0.0.1:5432/taskly",
		"APP_HOST":      "127.0.0.1",
		"APP_PORT":      "0",
		"BCRYPT_ROUNDS": "4",
		"LOG_SILENT":    "true",
		"LOG_TO_FILE":   "false",
	}
	for key, value := range overrides {
		env[key] = value
	}
	return env
}

func TestConfig(tb testing.TB, overrides map[string]string) config.Config {
	tb.Helper()
	env := TestEnv(overrides)
	cfg, err := config.Load(func(key string) string { return env[key] })
	if err != nil {
		tb.Fatalf("invalid test config: %v", err)
	}
	return cfg
}

func CreateTempPublicDir(tb testing.TB) string {
	tb.Helper()
	publicDir := tb.TempDir()
	avatarsDir := filepath.Join(publicDir, "images", "users")
	if err := os.MkdirAll(avatarsDir, 0o755); err != nil {
		tb.Fatal(err)
	}

	source, err := os.Open(filepath.Join(ProjectRoot(), "public", "images", "users", "default-avatar.jpg"))
	if err != nil {
		tb.Fatal(err)
	}
	defer source.Close()
	target, err := os.Create(filepath.Join(avatarsDir, "default-avatar.jpg"))
	if err != nil {
		tb.Fatal(err)
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		tb.Fatal(err)
	}
	return publicDir
}
