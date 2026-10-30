//go:build e2e

package e2e

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"taskly/backend/test/support"
)

var (
	apiBinary   string
	seedBinary  string
	listenLine  = regexp.MustCompile(`Server is running on http://127\.0\.0\.1:(\d+)`)
	startupWait = 20 * time.Second
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "taskly-e2e-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	apiBinary = filepath.Join(dir, "api")
	seedBinary = filepath.Join(dir, "seed")

	for target, output := range map[string]string{"./cmd/backend": apiBinary, "./cmd/seed": seedBinary} {
		build := exec.Command("go", "build", "-o", output, target)
		build.Dir = support.ProjectRoot()
		if out, err := build.CombinedOutput(); err != nil {
			fmt.Fprintf(os.Stderr, "build %s failed: %v\n%s", target, err, out)
			os.RemoveAll(dir)
			os.Exit(1)
		}
	}

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type process struct {
	cmd    *exec.Cmd
	stdout *syncBuffer
	stderr *syncBuffer
	exited chan struct{}
	err    error
}

type syncBuffer struct {
	buffer bytes.Buffer
	lines  chan string
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.buffer.Write(data)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		select {
		case b.lines <- scanner.Text():
		default:
		}
	}
	return len(data), nil
}

func environment(overrides map[string]string) []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	for key, value := range support.TestEnv(overrides) {
		env = append(env, key+"="+value)
	}
	return env
}

func start(t *testing.T, binary string, overrides map[string]string) *process {
	t.Helper()
	cmd := exec.Command(binary)
	cmd.Dir = t.TempDir()
	cmd.Env = environment(overrides)

	p := &process{
		cmd:    cmd,
		stdout: &syncBuffer{lines: make(chan string, 256)},
		stderr: &syncBuffer{lines: make(chan string, 256)},
		exited: make(chan struct{}),
	}
	cmd.Stdout = p.stdout
	cmd.Stderr = p.stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		p.err = cmd.Wait()
		close(p.exited)
	}()
	t.Cleanup(func() {
		select {
		case <-p.exited:
		default:
			_ = cmd.Process.Kill()
			<-p.exited
		}
	})
	return p
}

func (p *process) waitForPort(t *testing.T) int {
	t.Helper()
	deadline := time.After(startupWait)
	for {
		select {
		case line := <-p.stdout.lines:
			if match := listenLine.FindStringSubmatch(line); match != nil {
				port, _ := strconv.Atoi(match[1])
				return port
			}
		case <-p.exited:
			t.Fatalf("process exited early: %v\nstdout: %s\nstderr: %s", p.err, p.stdout.buffer.String(), p.stderr.buffer.String())
		case <-deadline:
			t.Fatalf("server did not start\nstdout: %s\nstderr: %s", p.stdout.buffer.String(), p.stderr.buffer.String())
		}
	}
}

func (p *process) wait(t *testing.T) int {
	t.Helper()
	select {
	case <-p.exited:
		if p.cmd.ProcessState == nil {
			return -1
		}
		return p.cmd.ProcessState.ExitCode()
	case <-time.After(startupWait):
		t.Fatalf("process did not exit\nstdout: %s\nstderr: %s", p.stdout.buffer.String(), p.stderr.buffer.String())
		return -1
	}
}

func serverEnv(t *testing.T) map[string]string {
	t.Helper()
	database := support.StartTestDatabase(t)
	return map[string]string{
		"DATABASE_URL": database.URL,
		"PUBLIC_DIR":   support.CreateTempPublicDir(t),
		"LOG_SILENT":   "false",
		"APP_LEVEL":    "production",
		"LOG_DIR":      t.TempDir(),
	}
}

func TestServerStartsServesAndShutsDownGracefully(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	port := process.waitForPort(t)

	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("status = %d", response.StatusCode)
	}

	if err := process.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code := process.wait(t); code != 0 {
		t.Fatalf("exit code = %d\nstdout: %s\nstderr: %s", code, process.stdout.buffer.String(), process.stderr.buffer.String())
	}
	if !strings.Contains(process.stdout.buffer.String(), "Shutdown complete") {
		t.Fatalf("missing shutdown log: %s", process.stdout.buffer.String())
	}
}

func TestServerShutsDownOnInterrupt(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	process.waitForPort(t)
	if err := process.cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	if code := process.wait(t); code != 0 {
		t.Fatalf("exit code = %d", code)
	}
}

func TestServerWritesJSONLogsToFileWhenEnabled(t *testing.T) {
	env := serverEnv(t)
	env["LOG_TO_FILE"] = "true"
	process := start(t, apiBinary, env)
	port := process.waitForPort(t)

	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	_ = process.cmd.Process.Signal(syscall.SIGTERM)
	process.wait(t)

	files, _ := filepath.Glob(filepath.Join(env["LOG_DIR"], "taskly-api-*.log*"))
	if len(files) == 0 {
		t.Fatal("no log file written")
	}
	content, _ := os.ReadFile(files[0])
	if !strings.Contains(string(content), `"msg":"HTTP request"`) {
		t.Fatalf("request log missing: %s", content)
	}
}

func TestServerRefusesToStartWithInvalidConfiguration(t *testing.T) {
	cases := map[string]struct {
		overrides map[string]string
		remove    string
		want      string
	}{
		"missing secret":   {remove: "AUTH_SECRET", want: "AUTH_SECRET"},
		"short secret":     {overrides: map[string]string{"AUTH_SECRET": "short"}, want: "at least 32"},
		"missing database": {remove: "DATABASE_URL", want: "DATABASE_URL"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cmd := exec.Command(apiBinary)
			cmd.Dir = t.TempDir()
			env := support.TestEnv(tc.overrides)
			if tc.remove != "" {
				delete(env, tc.remove)
			}
			cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
			for key, value := range env {
				cmd.Env = append(cmd.Env, key+"="+value)
			}
			var stderr bytes.Buffer
			cmd.Stderr = &stderr

			err := cmd.Run()
			exitErr, ok := err.(*exec.ExitError)
			if !ok || exitErr.ExitCode() == 0 {
				t.Fatalf("expected a non zero exit, got %v", err)
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr %q lacks %q", stderr.String(), tc.want)
			}
		})
	}
}

func TestServerFailsWhenTheDatabaseIsUnreachable(t *testing.T) {
	process := start(t, apiBinary, map[string]string{
		"DATABASE_URL":     "postgres://nobody:nothing@127.0.0.1:1/none",
		"DATABASE_TIMEOUT": "500",
	})
	if code := process.wait(t); code == 0 {
		t.Fatal("expected a failing exit code")
	}
}

func TestSeedPopulatesIdempotently(t *testing.T) {
	env := serverEnv(t)
	env["APP_LEVEL"] = "development"

	for range 2 {
		process := start(t, seedBinary, env)
		if code := process.wait(t); code != 0 {
			t.Fatalf("seed exit code = %d\nstderr: %s", code, process.stderr.buffer.String())
		}
	}

	server := start(t, apiBinary, env)
	port := server.waitForPort(t)
	client := support.NewClient(t, fmt.Sprintf("http://127.0.0.1:%d", port))
	login := client.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
		"email": "johndoe123@mail.com", "password": "password123",
	}})
	if login.Status != 200 {
		t.Fatalf("seeded user cannot sign in: %d %s", login.Status, login.Raw)
	}
	tasks := client.Get("/api/v1/tasks/user/" + login.String("_id"))
	if tasks.Body["taskCount"] != float64(1) {
		t.Fatalf("seeding twice must not duplicate tasks: %s", tasks.Raw)
	}
	users := client.Get("/api/v1/users")
	if len(users.List) != 2 {
		t.Fatalf("expected exactly two seeded users, got %d", len(users.List))
	}
}

func TestSeedRefusesToRunInProduction(t *testing.T) {
	env := serverEnv(t)
	env["APP_LEVEL"] = "production"
	process := start(t, seedBinary, env)
	if code := process.wait(t); code == 0 {
		t.Fatal("seed must refuse production")
	}
}

func TestUserJourney(t *testing.T) {
	process := start(t, apiBinary, serverEnv(t))
	port := process.waitForPort(t)
	client := support.NewClient(t, fmt.Sprintf("http://127.0.0.1:%d", port))

	creds, signup := support.RegisterUser(t, client)
	if signup.Status != 200 {
		t.Fatalf("signup failed: %s", signup.Raw)
	}
	id := signup.String("_id")

	created := client.Post("/api/v1/tasks/create", support.RequestOptions{JSON: support.TaskPayload(nil)})
	if created.Status != 200 {
		t.Fatalf("create failed: %s", created.Raw)
	}
	taskPath := "/api/v1/tasks/" + created.String("_id")

	patched := client.Patch(taskPath, support.RequestOptions{JSON: map[string]any{"status": "done"}})
	if patched.Status != 200 || patched.String("status") != "done" {
		t.Fatalf("update failed: %s", patched.Raw)
	}

	listed := client.Get("/api/v1/tasks/user/" + id + "?status=done")
	if listed.Body["taskCount"] != float64(1) {
		t.Fatalf("list failed: %s", listed.Raw)
	}

	if client.Get("/api/v1/auth/signout").Status != 200 {
		t.Fatal("sign out failed")
	}
	if client.Get(taskPath).Status != 401 {
		t.Fatal("signed out clients must be rejected")
	}

	login := client.Post("/api/v1/auth/signin", support.RequestOptions{JSON: map[string]any{
		"email": creds.Email, "password": creds.Password,
	}})
	if login.Status != 200 {
		t.Fatalf("sign in failed: %s", login.Raw)
	}
	if client.Delete("/api/v1/users/delete/"+id).Status != 200 {
		t.Fatal("account deletion failed")
	}
	if client.Get(taskPath).Status != 401 {
		t.Fatal("deleted accounts lose their session")
	}
}
