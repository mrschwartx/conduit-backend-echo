package support

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"strconv"
	"testing"

	"taskly/backend/internal/bootstrap"
)

type App struct {
	*bootstrap.Server
	BaseURL   string
	PublicDir string
	tb        testing.TB
}

func StartApp(tb testing.TB, databaseURL string, overrides map[string]string) *App {
	tb.Helper()
	publicDir := CreateTempPublicDir(tb)

	values := map[string]string{"DATABASE_URL": databaseURL, "PUBLIC_DIR": publicDir}
	for key, value := range overrides {
		values[key] = value
	}
	cfg := TestConfig(tb, values)

	server, err := bootstrap.Start(context.Background(), cfg, slog.New(slog.DiscardHandler))
	if err != nil {
		tb.Fatalf("cannot start the app: %v", err)
	}
	app := &App{
		Server:    server,
		BaseURL:   "http://127.0.0.1:" + strconv.Itoa(server.Port()),
		PublicDir: publicDir,
		tb:        tb,
	}
	tb.Cleanup(func() {
		if err := server.Stop(); err != nil {
			tb.Errorf("stop failed: %v", err)
		}
	})
	return app
}

func (a *App) NewClient() *Client {
	return NewClient(a.tb, a.BaseURL)
}

type Credentials struct {
	Username string
	Email    string
	Password string
}

func RandomCredentials() Credentials {
	return Credentials{
		Username: "user_" + randomToken(8),
		Email:    randomToken(8) + "@mail.com",
		Password: "Password123!",
	}
}

func RegisterUser(tb testing.TB, client *Client, mutate ...func(*Credentials)) (Credentials, Response) {
	tb.Helper()
	credentials := RandomCredentials()
	for _, change := range mutate {
		change(&credentials)
	}
	response := client.Post("/api/v1/auth/signup", RequestOptions{JSON: map[string]any{
		"username": credentials.Username,
		"email":    credentials.Email,
		"password": credentials.Password,
	}})
	return credentials, response
}

func MustRegisterUser(tb testing.TB, client *Client) (Credentials, string) {
	tb.Helper()
	credentials, response := RegisterUser(tb, client)
	if response.Status != 200 {
		tb.Fatalf("signup failed: %d %s", response.Status, response.Raw)
	}
	return credentials, response.String("_id")
}

func TaskPayload(overrides map[string]any) map[string]any {
	payload := map[string]any{
		"name":        "Read Atomic Habits",
		"description": "Finish reading Atomic Habits by James Clear",
		"priority":    "low",
		"due":         "2026-12-08T10:00:00Z",
		"status":      "todo",
	}
	for key, value := range overrides {
		if value == nil {
			delete(payload, key)
			continue
		}
		payload[key] = value
	}
	return payload
}

func randomToken(length int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	token := make([]byte, length)
	for index := range token {
		token[index] = alphabet[rand.IntN(len(alphabet))]
	}
	return string(token)
}
