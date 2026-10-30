//go:build integration

package integration

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"taskly/backend/test/support"
)

const missingID = "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee"

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$`)

func expectStatus(t *testing.T, response support.Response, want int) {
	t.Helper()
	if response.Status != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Status, want, response.Raw)
	}
}

func expectEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func expectMessage(t *testing.T, response support.Response, want string) {
	t.Helper()
	expectEqual(t, response.String("message"), want)
}

func avatarFiles(t *testing.T, publicDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(publicDir, "images", "users"))
	if err != nil {
		t.Fatal(err)
	}
	files := []string{}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "avatar-") {
			files = append(files, entry.Name())
		}
	}
	return files
}

type signedIn struct {
	client *support.Client
	id     string
	creds  support.Credentials
}

func signIn(t *testing.T, app *support.App) signedIn {
	t.Helper()
	client := app.NewClient()
	creds, id := support.MustRegisterUser(t, client)
	return signedIn{client: client, id: id, creds: creds}
}
