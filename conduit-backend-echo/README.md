# Conduit Backend (Go + Echo v5 + MySQL 8)

One of several interchangeable implementations of the Conduit API contract
(`../conduit-openapi/openapi.yml`, base URL `http://localhost:8080/api/v1`).

## Prerequisites
- Go (see `go.mod`), Docker (MySQL + integration tests)
- First time only, with network access, resolve dependencies:

```bash
cd conduit-backend-echo
go get github.com/labstack/echo/v5@latest
go mod tidy          # creates go.sum
go build ./... && go vet ./...
```

## Run the database
```bash
docker compose -f ../conduit-docker/docker-compose.local.yml up -d conduit-mysql
```

## Run the app (single command)
```bash
export JWT_SECRET="$(openssl rand -hex 32)"   # required, >= 32 chars
go run ./cmd/api
```
Defaults match `docker-compose.local.yml`; migrations are applied at startup
(`DATABASE_RUN_MIGRATIONS=false` disables that). Check: `curl localhost:8080/health` -> `OK`.

Everything in Docker: `docker compose -f ../conduit-docker/docker-compose.local.yml up --build`.

## Tests
```bash
go test ./...                         # unit + integration (Testcontainers starts mysql:8.4)
# or against an existing MySQL:
TEST_DATABASE_HOST=localhost TEST_DATABASE_NAME=conduit_test \
TEST_DATABASE_USER=root TEST_DATABASE_PASSWORD=... go test ./...
```
Without Docker or `TEST_DATABASE_*`, the integration tests are skipped (unit tests still run).
Contract tests (if `../conduit-contract-tests` exists): `API_URL=http://localhost:8080/api/v1 <runner>`.

## Environment variables
See `.env.example`. Key ones: `PORT`, `DATABASE_*`, `JWT_SECRET` (required),
`JWT_TTL_MINUTES`, `CORS_ALLOWED_ORIGINS`, `LOG_LEVEL`, `BCRYPT_COST`.

## Layout
```
cmd/api/main.go            entry point, graceful shutdown
internal/identity|profiles|articles|comments   model, dto, repository, service, handler
internal/platform/{config,database,logger,httpx,auth,validation}
internal/server            router + middleware wiring (also used by integration tests)
migrations/*.sql           embedded golang-migrate files
```
