# Notes

## Verification status (IMPORTANT)
This code was generated in an environment **without a Go toolchain, network or Docker**.
It has NOT been compiled or run. Echo v5, testcontainers and golang-migrate APIs were
written from memory and must be verified. First steps:
1. `go get github.com/labstack/echo/v5@latest && go mod tidy && go build ./... && go vet ./...`
2. Fix any v5 API mismatch. The Echo v5 surface used is intentionally small:
   `echo.New`, `*echo.Context` handlers (`c.Request/Response/Param/QueryParam/Get/Set`),
   `echo.HandlerFunc`, `echo.MiddlewareFunc`, `e.Group`, route registration with
   per-route middleware, `e.HTTPErrorHandler func(c *echo.Context, err error)`,
   `*echo.HTTPError{Code}`, and `*echo.Echo` as an `http.Handler`.
   All cross-cutting middleware (request id, access log, recover, CORS, secure headers,
   body limit) is plain `net/http`, so it does not depend on Echo's middleware package.
3. `go test ./...` (testcontainers: `tcmysql.Run`, `testcontainers.TerminateContainer`).

## Dependencies (non-obvious)
- `golang.org/x/text` (norm): NFKD normalisation for slugs.
- `golang-migrate/v4` + `iofs`: versioned SQL migrations embedded in the binary.
- `testcontainers-go` + `modules/mysql`: real MySQL for integration tests.

## Spec ambiguities / defects and assumptions
1. openapi.yml does not declare 400/403/409/415 on most operations; the prompt requires them. Implemented as in the prompt.
2. `POST /articles` declares 409 in the spec, but slug collisions get `-2`, `-3`..., so it never occurs.
3. Spec examples use key `resource`; implemented keys are `article`, `comment`, `profile`.
4. Spec has no max for `limit` and no default for `offset`; implemented: default 0, limit default 20, clamp 50.
5. Optional-auth GETs list no 401 in the spec; an invalid/expired token returns 401 per the prompt.
6. `Token` scheme is only described in prose in the spec (apiKey header); only `Token <jwt>` is accepted.
7. Password rules on `PUT /user` are not specified: same as registration (8-72 characters, and <= 72 bytes for bcrypt).
8. Valid JWT for a nonexistent user -> 401 `token: ["is invalid"]` (checked on GET/PUT /user; no user deletion exists in the contract).
9. Passwords are NOT trimmed (trimming would change the secret); all other strings are.
10. Non-integer comment id in the path -> 422 `errors.id = ["is invalid"]`.
11. `null` for `title/description/body/email/username/password` on update -> 422 "can't be blank"; `tagList: null` clears all tags.
12. Body parsing (400/415) happens before existence/ownership checks (404/403); a JSON type mismatch (e.g. `"title": 5`) is 422 `errors.title = ["is invalid"]`.
13. Unknown routes / wrong methods use the contract format: `errors.route = ["not found" | "method not allowed"]`. Oversized bodies: 413 `errors.body`.
14. Unknown JSON fields are ignored (lenient) so frontends sending extras keep working.
15. Slug and username lookups are case-insensitive (ci collation); tags are matched exactly (utf8mb4_bin).
16. A slug that is a single dash-free base is truncated so that base + suffix never exceeds 80 chars.
17. Not done: git commits per step (no repo access), generated code (none needed), contract-test run (directory absent).
