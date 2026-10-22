package server_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	"conduit/backend/internal/platform/config"
	"conduit/backend/internal/platform/database"
	"conduit/backend/internal/server"
)

var (
	srv    *httptest.Server
	testDB *sql.DB
)

func TestMain(m *testing.M) { os.Exit(run(m)) }

func run(m *testing.M) int {
	ctx := context.Background()
	cfg, cleanup, err := testDatabase(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration tests disabled:", err)
		return m.Run() // tests call need(t) and skip
	}
	defer cleanup()

	if err := database.Migrate(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		return 1
	}
	testDB, err = database.Open(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "open:", err)
		return 1
	}
	defer testDB.Close()

	h, err := server.New(server.Deps{
		DB:          testDB,
		Logger:      slog.New(slog.NewJSONHandler(io.Discard, nil)),
		JWTSecret:   "0123456789abcdef0123456789abcdef",
		JWTTTL:      time.Hour,
		BcryptCost:  4,
		CORSOrigins: []string{"http://localhost:5173"},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		return 1
	}
	srv = httptest.NewServer(h)
	defer srv.Close()
	return m.Run()
}

// testDatabase uses TEST_DATABASE_* env vars when set, else starts MySQL 8 via Testcontainers.
func testDatabase(ctx context.Context) (config.Database, func(), error) {
	pool := func(c config.Database) config.Database {
		c.MaxOpenConns, c.MaxIdleConns, c.ConnMaxLifetime = 10, 5, time.Minute
		return c
	}
	if host := os.Getenv("TEST_DATABASE_HOST"); host != "" {
		port, _ := strconv.Atoi(os.Getenv("TEST_DATABASE_PORT"))
		if port == 0 {
			port = 3306
		}
		return pool(config.Database{
			Host: host, Port: port,
			Name:     os.Getenv("TEST_DATABASE_NAME"),
			User:     os.Getenv("TEST_DATABASE_USER"),
			Password: os.Getenv("TEST_DATABASE_PASSWORD"),
		}), func() {}, nil
	}

	ctr, err := tcmysql.Run(ctx, "mysql:8.4",
		tcmysql.WithDatabase("conduit_test"),
		tcmysql.WithUsername("test"),
		tcmysql.WithPassword("test"),
	)
	if err != nil {
		return config.Database{}, nil, err
	}
	host, err := ctr.Host(ctx)
	if err != nil {
		return config.Database{}, nil, err
	}
	port, err := ctr.MappedPort(ctx, "3306/tcp")
	if err != nil {
		return config.Database{}, nil, err
	}
	return pool(config.Database{
			Host: host, Port: port.Int(), Name: "conduit_test", User: "test", Password: "test",
		}),
		func() { _ = testcontainers.TerminateContainer(ctr) }, nil
}

// need skips a test when no database is available and wipes all tables.
func need(t *testing.T) {
	t.Helper()
	if srv == nil {
		t.Skip("no database available (Docker or TEST_DATABASE_* required)")
	}
	for _, tbl := range []string{"comments", "favorites", "article_tags", "articles", "tags", "follows", "users"} {
		_, err := testDB.Exec("DELETE FROM " + tbl)
		require.NoError(t, err)
	}
}

// ---- helpers ---------------------------------------------------------------

type response struct {
	status int
	header http.Header
	body   map[string]any
	raw    string
}

func send(t *testing.T, req *http.Request) response {
	t.Helper()
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	r := response{status: res.StatusCode, header: res.Header, raw: string(raw)}
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &r.body), string(raw))
	}
	return r
}

// do calls /api/v1+path. A string body is sent raw; anything else is JSON-encoded.
// Content-Type is only set when there is a body.
func do(t *testing.T, method, path, token string, body any) response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			rdr = strings.NewReader(s)
		} else {
			raw, err := json.Marshal(body)
			require.NoError(t, err)
			rdr = bytes.NewReader(raw)
		}
	}
	req, err := http.NewRequest(method, srv.URL+"/api/v1"+path, rdr)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Token "+token)
	}
	return send(t, req)
}

func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any         { l, _ := v.([]any); return l }
func str(v any) string         { s, _ := v.(string); return s }

type m = map[string]any

func register(t *testing.T, username string) string {
	t.Helper()
	r := do(t, "POST", "/users", "", m{"user": m{
		"username": username, "email": username + "@example.com", "password": "password123"}})
	require.Equal(t, 201, r.status, r.raw)
	return str(obj(r.body["user"])["token"])
}

func createArticle(t *testing.T, token, title string, tags ...string) string {
	t.Helper()
	if tags == nil {
		tags = []string{}
	}
	r := do(t, "POST", "/articles", token, m{"article": m{
		"title": title, "description": "desc of " + title, "body": "body of " + title, "tagList": tags}})
	require.Equal(t, 201, r.status, r.raw)
	return str(obj(r.body["article"])["slug"])
}

func errorsOf(r response) map[string]any { return obj(r.body["errors"]) }

// ---- tests -----------------------------------------------------------------

func TestHealth(t *testing.T) {
	need(t)
	res, err := http.Get(srv.URL + "/health")
	require.NoError(t, err)
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "OK", string(raw))
	assert.NotEmpty(t, res.Header.Get("X-Request-Id"))
	assert.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
}

func TestUnknownRouteUsesContractError(t *testing.T) {
	need(t)
	r := do(t, "GET", "/nope", "", nil)
	assert.Equal(t, 404, r.status)
	assert.Contains(t, errorsOf(r), "route")
}

func TestUserLifecycle(t *testing.T) {
	need(t)

	r := do(t, "POST", "/users", "", m{"user": m{"username": "alice", "email": "Alice@Example.com", "password": "password123"}})
	require.Equal(t, 201, r.status, r.raw)
	u := obj(r.body["user"])
	assert.Equal(t, "alice@example.com", u["email"], "email stored lowercase")
	assert.NotEmpty(t, u["token"])
	bio, hasBio := u["bio"]
	assert.True(t, hasBio, "bio key must be present")
	assert.Nil(t, bio)
	assert.NotContains(t, r.raw, "password")

	// duplicates (case-insensitive) -> 409
	r = do(t, "POST", "/users", "", m{"user": m{"username": "ALICE", "email": "ALICE@example.com", "password": "password123"}})
	assert.Equal(t, 409, r.status)
	assert.Contains(t, errorsOf(r), "username")
	assert.Contains(t, errorsOf(r), "email")

	// validation reports all fields at once
	r = do(t, "POST", "/users", "", m{"user": m{"username": "a", "email": "nope", "password": "x"}})
	assert.Equal(t, 422, r.status)
	for _, f := range []string{"username", "email", "password"} {
		assert.Contains(t, errorsOf(r), f)
	}

	// login (case-insensitive email) / wrong password / unknown email
	r = do(t, "POST", "/users/login", "", m{"user": m{"email": "ALICE@example.com", "password": "password123"}})
	assert.Equal(t, 200, r.status, r.raw)
	token := str(obj(r.body["user"])["token"])
	for _, cred := range []m{{"email": "alice@example.com", "password": "wrongpass1"}, {"email": "ghost@example.com", "password": "password123"}} {
		r = do(t, "POST", "/users/login", "", m{"user": cred})
		assert.Equal(t, 401, r.status)
		assert.Equal(t, []any{"are invalid"}, errorsOf(r)["credentials"])
	}

	// auth errors
	r = do(t, "GET", "/user", "", nil)
	assert.Equal(t, 401, r.status)
	assert.Equal(t, []any{"is missing"}, errorsOf(r)["token"])

	req, _ := http.NewRequest("GET", srv.URL+"/api/v1/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r = send(t, req)
	assert.Equal(t, 401, r.status, "Bearer scheme must be rejected")
	assert.Equal(t, []any{"is invalid"}, errorsOf(r)["token"])

	r = do(t, "GET", "/user", token, nil)
	assert.Equal(t, 200, r.status)

	// partial update: absent vs null vs value
	r = do(t, "PUT", "/user", token, m{"user": m{"bio": "hello"}})
	require.Equal(t, 200, r.status, r.raw)
	assert.Equal(t, "hello", obj(r.body["user"])["bio"])
	assert.Equal(t, "alice", obj(r.body["user"])["username"], "absent fields unchanged")

	r = do(t, "PUT", "/user", token, m{"user": m{"bio": nil}})
	assert.Nil(t, obj(r.body["user"])["bio"], "null clears bio")

	r = do(t, "PUT", "/user", token, m{"user": m{}})
	assert.Equal(t, 422, r.status, "empty update is rejected")

	r = do(t, "PUT", "/user", token, m{"user": m{"image": "http://insecure.example/x.png"}})
	assert.Equal(t, 422, r.status)

	// uniqueness on update ignores own record, conflicts with others
	register(t, "bob")
	r = do(t, "PUT", "/user", token, m{"user": m{"username": "Alice"}})
	assert.Equal(t, 200, r.status, "own record is ignored")
	r = do(t, "PUT", "/user", token, m{"user": m{"username": "bob"}})
	assert.Equal(t, 409, r.status)
}

func TestBodyErrors(t *testing.T) {
	need(t)

	req, _ := http.NewRequest("POST", srv.URL+"/api/v1/users", strings.NewReader(`{"user":{}}`))
	req.Header.Set("Content-Type", "text/plain")
	assert.Equal(t, 415, send(t, req).status)

	r := do(t, "POST", "/users", "", `{"user":`)
	assert.Equal(t, 400, r.status)
	assert.Equal(t, []any{"is not valid JSON"}, errorsOf(r)["body"])

	// bodyless requests must not require Content-Type
	token := register(t, "alice")
	slug := createArticle(t, token, "To delete")
	r = do(t, "DELETE", "/articles/"+slug, token, nil)
	assert.Equal(t, 204, r.status)
	assert.Empty(t, r.raw)
}

func TestArticleLifecycleAndSlugs(t *testing.T) {
	need(t)
	alice := register(t, "alice")
	bob := register(t, "bob")

	s1 := createArticle(t, alice, "Hello World", "go", "web")
	s2 := createArticle(t, alice, "Hello World")
	s3 := createArticle(t, alice, "Feed")
	assert.Equal(t, "hello-world", s1)
	assert.Equal(t, "hello-world-2", s2)
	assert.Equal(t, "feed-2", s3, "'feed' is reserved")

	r := do(t, "GET", "/articles/"+s1, "", nil)
	require.Equal(t, 200, r.status)
	a := obj(r.body["article"])
	assert.Equal(t, []any{"go", "web"}, a["tagList"])
	assert.Contains(t, a, "body")
	assert.Equal(t, false, a["favorited"])
	assert.EqualValues(t, 0, a["favoritesCount"])
	assert.Equal(t, "alice", obj(a["author"])["username"])
	assert.Regexp(t, `^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d\.\d{3}Z$`, a["createdAt"])

	// 403 for non-owner, 404 for unknown, 422 for invalid
	r = do(t, "PUT", "/articles/"+s1, bob, m{"article": m{"title": "Hacked"}})
	assert.Equal(t, 403, r.status)
	assert.Equal(t, []any{"forbidden"}, errorsOf(r)["article"])
	r = do(t, "PUT", "/articles/missing", bob, m{"article": m{"title": "x"}})
	assert.Equal(t, 404, r.status)
	r = do(t, "PUT", "/articles/"+s1, alice, m{"article": m{"title": ""}})
	assert.Equal(t, 422, r.status)

	// title change regenerates slug; unchanged title keeps it; other fields stay
	r = do(t, "PUT", "/articles/"+s1, alice, m{"article": m{"title": "Brand New Title"}})
	require.Equal(t, 200, r.status, r.raw)
	a = obj(r.body["article"])
	assert.Equal(t, "brand-new-title", a["slug"])
	assert.Equal(t, "body of Hello World", a["body"])
	assert.Equal(t, []any{"go", "web"}, a["tagList"])
	r = do(t, "PUT", "/articles/brand-new-title", alice, m{"article": m{"title": "Brand New Title", "tagList": []string{"x"}}})
	assert.Equal(t, "brand-new-title", obj(r.body["article"])["slug"])
	assert.Equal(t, []any{"x"}, obj(r.body["article"])["tagList"])

	// validation: all failing fields at once
	r = do(t, "POST", "/articles", alice, m{"article": m{"title": "", "description": "", "body": "", "tagList": []string{"a b"}}})
	assert.Equal(t, 422, r.status)
	for _, f := range []string{"title", "description", "body", "tagList"} {
		assert.Contains(t, errorsOf(r), f)
	}

	// delete cascades to comments
	do(t, "POST", "/articles/"+s2+"/comments", bob, m{"comment": m{"body": "hi"}})
	assert.Equal(t, 204, do(t, "DELETE", "/articles/"+s2, alice, nil).status)
	assert.Equal(t, 404, do(t, "GET", "/articles/"+s2, "", nil).status)
	var n int
	require.NoError(t, testDB.QueryRow("SELECT COUNT(*) FROM comments").Scan(&n))
	assert.Equal(t, 0, n)
}

func TestFeedRouteWinsOverSlug(t *testing.T) {
	need(t)
	alice := register(t, "alice")
	r := do(t, "GET", "/articles/feed", alice, nil)
	assert.Equal(t, 200, r.status, r.raw)
	assert.Contains(t, r.body, "articles")
	assert.Contains(t, r.body, "articlesCount")

	// requires auth, so it is not served by the optional-auth article route
	assert.Equal(t, 401, do(t, "GET", "/articles/feed", "", nil).status)
}

func TestListFiltersFeedFavoritesAndFollow(t *testing.T) {
	need(t)
	alice := register(t, "alice")
	bob := register(t, "bob")

	a1 := createArticle(t, alice, "First", "go", "web")
	a2 := createArticle(t, alice, "Second", "web")
	createArticle(t, bob, "Bobs")

	// anonymous list, newest first, items omit body
	r := do(t, "GET", "/articles", "", nil)
	require.Equal(t, 200, r.status)
	items := list(r.body["articles"])
	assert.Len(t, items, 3)
	assert.EqualValues(t, 3, r.body["articlesCount"])
	assert.NotContains(t, obj(items[0]), "body")
	assert.Equal(t, "bobs", obj(items[0])["slug"])

	// filters
	assert.EqualValues(t, 1, do(t, "GET", "/articles?tag=go", "", nil).body["articlesCount"])
	assert.EqualValues(t, 2, do(t, "GET", "/articles?author=alice", "", nil).body["articlesCount"])
	r = do(t, "GET", "/articles?author=nobody", "", nil)
	assert.Equal(t, 200, r.status)
	assert.EqualValues(t, 0, r.body["articlesCount"])
	assert.Equal(t, []any{}, r.body["articles"])
	assert.Equal(t, 200, do(t, "GET", "/articles?favorited=nobody", "", nil).status)

	// pagination: count ignores offset/limit; limit clamps; invalid -> 422
	r = do(t, "GET", "/articles?limit=1&offset=1", "", nil)
	assert.Len(t, list(r.body["articles"]), 1)
	assert.EqualValues(t, 3, r.body["articlesCount"])
	assert.Equal(t, 200, do(t, "GET", "/articles?limit=1000", "", nil).status)
	assert.Equal(t, 422, do(t, "GET", "/articles?limit=0", "", nil).status)
	assert.Equal(t, 422, do(t, "GET", "/articles?offset=-1", "", nil).status)
	assert.Equal(t, 422, do(t, "GET", "/articles?limit=abc", "", nil).status)

	// follow / feed
	r = do(t, "POST", "/profiles/alice/follow", bob, nil)
	require.Equal(t, 200, r.status, r.raw)
	assert.Equal(t, true, obj(r.body["profile"])["following"])
	assert.Equal(t, 200, do(t, "POST", "/profiles/alice/follow", bob, nil).status, "idempotent")
	assert.EqualValues(t, 2, do(t, "GET", "/articles/feed", bob, nil).body["articlesCount"])
	assert.EqualValues(t, 0, do(t, "GET", "/articles/feed", alice, nil).body["articlesCount"])
	assert.Equal(t, true, obj(do(t, "GET", "/profiles/alice", bob, nil).body["profile"])["following"])
	assert.Equal(t, false, obj(do(t, "GET", "/profiles/alice", "", nil).body["profile"])["following"])
	assert.Equal(t, 404, do(t, "GET", "/profiles/ghost", "", nil).status)

	r = do(t, "POST", "/profiles/bob/follow", bob, nil)
	assert.Equal(t, 422, r.status)
	assert.Equal(t, []any{"you cannot follow yourself"}, errorsOf(r)["username"])

	r = do(t, "DELETE", "/profiles/alice/follow", bob, nil)
	assert.Equal(t, false, obj(r.body["profile"])["following"])

	// favorites
	r = do(t, "POST", "/articles/"+a1+"/favorite", bob, nil)
	require.Equal(t, 200, r.status, r.raw)
	assert.Equal(t, true, obj(r.body["article"])["favorited"])
	assert.EqualValues(t, 1, obj(r.body["article"])["favoritesCount"])
	r = do(t, "POST", "/articles/"+a1+"/favorite", bob, nil)
	assert.EqualValues(t, 1, obj(r.body["article"])["favoritesCount"], "idempotent")
	assert.EqualValues(t, 1, do(t, "GET", "/articles?favorited=bob", "", nil).body["articlesCount"])
	assert.Equal(t, false, obj(do(t, "GET", "/articles/"+a1, "", nil).body["article"])["favorited"])
	assert.Equal(t, true, obj(do(t, "GET", "/articles/"+a1, bob, nil).body["article"])["favorited"])
	r = do(t, "DELETE", "/articles/"+a1+"/favorite", bob, nil)
	assert.EqualValues(t, 0, obj(r.body["article"])["favoritesCount"])
	assert.Equal(t, 200, do(t, "DELETE", "/articles/"+a1+"/favorite", bob, nil).status, "idempotent")
	assert.Equal(t, 404, do(t, "POST", "/articles/missing/favorite", bob, nil).status)
	_ = a2
}

func TestComments(t *testing.T) {
	need(t)
	alice := register(t, "alice")
	bob := register(t, "bob")
	slug := createArticle(t, alice, "Commented")
	other := createArticle(t, alice, "Other")

	r := do(t, "POST", "/articles/"+slug+"/comments", bob, m{"comment": m{"body": "first"}})
	require.Equal(t, 201, r.status, r.raw)
	c1 := obj(r.body["comment"])
	assert.IsType(t, float64(0), c1["id"], "id is an integer")
	assert.Equal(t, "bob", obj(c1["author"])["username"])
	do(t, "POST", "/articles/"+slug+"/comments", alice, m{"comment": m{"body": "second"}})

	// newest first
	r = do(t, "GET", "/articles/"+slug+"/comments", "", nil)
	cs := list(r.body["comments"])
	require.Len(t, cs, 2)
	assert.Equal(t, "second", obj(cs[0])["body"])

	// validation / 404
	assert.Equal(t, 422, do(t, "POST", "/articles/"+slug+"/comments", bob, m{"comment": m{"body": " "}}).status)
	assert.Equal(t, 404, do(t, "POST", "/articles/missing/comments", bob, m{"comment": m{"body": "x"}}).status)
	assert.Equal(t, 404, do(t, "GET", "/articles/missing/comments", "", nil).status)
	assert.Equal(t, 401, do(t, "POST", "/articles/"+slug+"/comments", "", m{"comment": m{"body": "x"}}).status)

	id := strconv.Itoa(int(c1["id"].(float64)))
	// 403 non-author, 404 wrong slug, 204 author
	r = do(t, "DELETE", "/articles/"+slug+"/comments/"+id, alice, nil)
	assert.Equal(t, 403, r.status)
	assert.Equal(t, []any{"forbidden"}, errorsOf(r)["comment"])
	assert.Equal(t, 404, do(t, "DELETE", "/articles/"+other+"/comments/"+id, bob, nil).status)
	assert.Equal(t, 404, do(t, "DELETE", "/articles/"+slug+"/comments/999999", bob, nil).status)
	assert.Equal(t, 204, do(t, "DELETE", "/articles/"+slug+"/comments/"+id, bob, nil).status)
	assert.Len(t, list(do(t, "GET", "/articles/"+slug+"/comments", "", nil).body["comments"]), 1)
}

func TestTagsRanking(t *testing.T) {
	need(t)
	alice := register(t, "alice")
	createArticle(t, alice, "One", "beta", "alpha")
	createArticle(t, alice, "Two", "beta", "gamma")
	createArticle(t, alice, "Three", "beta", "alpha")

	r := do(t, "GET", "/tags", "", nil)
	require.Equal(t, 200, r.status)
	// beta(3) > alpha(2) > gamma(1)
	assert.Equal(t, []any{"beta", "alpha", "gamma"}, r.body["tags"])
}

func TestCORSPreflight(t *testing.T) {
	need(t)
	req, _ := http.NewRequest("OPTIONS", srv.URL+"/api/v1/articles", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	res, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	assert.Equal(t, 204, res.StatusCode)
	assert.Equal(t, "http://localhost:5173", res.Header.Get("Access-Control-Allow-Origin"))
	assert.Contains(t, res.Header.Get("Access-Control-Allow-Headers"), "Authorization")

	req.Header.Set("Origin", "http://evil.example")
	res2, err := srv.Client().Do(req)
	require.NoError(t, err)
	defer res2.Body.Close()
	assert.Empty(t, res2.Header.Get("Access-Control-Allow-Origin"))
}
