# Taskly Backend (Echo)

A task manager backend: sign up and sign in, manage your profile and avatar, and create, filter, sort and paginate to-do tasks. It is a REST API built with Go, Echo v5 and PostgreSQL, organised as a modular monolith with Domain-Driven Design and Clean Architecture.

The browser client is a separate Taskly frontend that talks to this API. The HTTP contract is the same as the former Express version, so the frontend needs no changes.

**Stack:** Go 1.27 · Echo v5 · PostgreSQL 18 · pgx · JWT · bcrypt · log/slog · testing · Docker · GitHub Actions

## Contents

- [Quick start](#quick-start)
- [Features](#features)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Database](#database)
- [API](#api)
- [Testing](#testing)
- [CI](#ci)
- [Limits](#limits)
- [Migrated from Express](#migrated-from-express)
- [References](#references)

## Quick start

Requires Go 1.27+ and Docker.

```sh
cd taskly-docker
make docker/dev-up
cd ../taskly-backend
cp .env.example .env
make secret-key
make run
```

Paste the generated value into `AUTH_SECRET` in `.env`. The API runs at http://localhost:8080 and pgAdmin at http://localhost:8081 (login `admin@taskly.dev` / `admin`, server host `taskly-postgres`, user `dev_user`, password `dev_pass`). Migrations run automatically on startup. The `.env` file is loaded when present, and real environment variables always take precedence.

| Goal               | Command                                   |
| ------------------ | ----------------------------------------- |
| Run in development | `make run`                                |
| Build binaries     | `make build` (writes to `bin/`)           |
| Apply migrations   | `make migrate`                            |
| Seed demo data     | `make seed`                               |
| Run with Docker    | `make docker/up` (from `taskly-docker`)   |
| Seed in Docker     | `make docker/seed` (from `taskly-docker`) |
| Stop Docker        | `make docker/down` (from `taskly-docker`) |

`make docker/up` creates `taskly-docker/.env` with a random database password and `AUTH_SECRET` on the first run, then starts PostgreSQL and the API and waits until both are healthy. `PORT` overrides the host port in the Docker setup. The container always listens on port 8080. Demo login after seeding: `johndoe123@mail.com` / `password123`.

## Features

- **Authentication**: sign up, sign in and sign out with a JWT stored in an HTTP-only cookie.
- **Users**: list users, update the profile, upload an avatar and delete the account.
- **Tasks**: create, view, update and delete tasks.
- **Listing**: filter by status, sort and paginate tasks.
- **Platform**: versioned SQL migrations embedded in the binary, structured logging with daily rotation, graceful shutdown on `SIGINT` and `SIGTERM`, health check on `/` in the Docker image.

## Configuration

| Variable            | Default                 | Purpose                                  |
| ------------------- | ----------------------- | ---------------------------------------- |
| `APP_NAME`          | `taskly-api`            | Application name                         |
| `APP_HOST`          | `0.0.0.0`               | Bind address                             |
| `APP_PORT`          | `8080`                  | Port the API listens on                  |
| `APP_LEVEL`         | `development`           | Environment level                        |
| `DATABASE_URL`      | required                | PostgreSQL connection string             |
| `DATABASE_POOL_MAX` | `10`                    | Maximum connections in the pool          |
| `DATABASE_TIMEOUT`  | `30000`                 | Connection timeout in milliseconds       |
| `AUTH_SECRET`       | required                | JWT secret, at least 32 characters       |
| `COOKIE_DOMAIN`     | empty                   | Domain of the auth cookie (production)   |
| `BCRYPT_ROUNDS`     | `10`                    | bcrypt cost factor                       |
| `CLIENT_URL`        | `http://localhost:3000` | Allowed CORS origin                      |
| `LOG_LEVEL`         | `debug`                 | Log level (`info` in production)         |
| `LOG_TO_FILE`       | `true`                  | Write logs to the `logs` directory       |
| `LOG_DIR`           | `logs`                  | Directory for log files                  |
| `LOG_SILENT`        | `false`                 | Disable all logging                      |
| `PUBLIC_DIR`        | `public`                | Directory with static files and avatars  |
| `SEED_ALLOW_PRODUCTION` | `false`             | Allow `seed` when `APP_LEVEL=production` |

Docker only, in `taskly-docker/.env`:

| Variable      | Default  | Purpose                                 |
| ------------- | -------- | --------------------------------------- |
| `PORT`        | `8080`   | Host port used by Docker                |
| `DB_NAME`     | `taskly` | PostgreSQL database name                |
| `DB_USER`     | `taskly` | PostgreSQL user                         |
| `DB_PASSWORD` | required | PostgreSQL password, generated by make |

The frontend must set `VITE_API_BASE_URL=http://localhost:8080/api/v1` and `VITE_SERVER_BASE_URL=http://localhost:8080`.

## Architecture

### Project layout

```
taskly-backend/
  cmd/
    backend/     HTTP server entrypoint
    migrate/     applies pending migrations
    seed/        demo data
  internal/
    bootstrap/   composition root, app and server factories
    config/      environment parsing and validation
    shared/      errors, event bus, logging, HTTP and PostgreSQL helpers
    modules/     users, tasks
  test/
    support/     helpers shared by integration and e2e suites
    integration/ repositories and HTTP modules on a real PostgreSQL
    e2e/         built binaries run as real processes
taskly-docker/
  taskly-backend/Dockerfile
  docker-compose.local.yml
  docker-compose.dev.yml
  Makefile
.github/workflows/ci.yml
```

### Module layout

```
internal/modules/<name>/
  domain/                  entities, value objects, repository contracts
  application/             use cases
  infrastructure/          PostgreSQL and other adapters
  presentation/rest/       Echo handlers, middleware and routes
  module.go                wires the module and registers its routes
```

### Dependency rules

- Dependencies point inward: presentation and infrastructure depend on application, application depends on domain.
- Modules never import each other. They communicate through the in-process event bus, so deleting a user removes that user's tasks. The tasks module only receives the authentication middleware as a dependency.
- Every I/O call takes a `context.Context`, so client disconnects and shutdown propagate to the database.

## Database

Schema changes live in `internal/shared/infrastructure/postgres/migrations` as numbered `.sql` files and are embedded in the binaries. The migrator applies pending files in order, each inside a transaction, records them in `schema_migrations` and holds an advisory lock so concurrent starts do not race. It runs on every startup and through `make migrate`.

| Table   | Notes                                                                                   |
| ------- | --------------------------------------------------------------------------------------- |
| `users` | `id` UUID, case-insensitive unique `email` and `username`, `password_hash`, `avatar_file`, timestamps |
| `tasks` | `id` UUID, `owner_id` UUID (foreign key to `users`), `due` timestamptz, `status` and `priority` limited by `CHECK`, indexed on `(owner_id, status)` |

- Identifiers are UUIDs. API responses expose them as `_id` and `owner`, so existing clients keep working.
- Listings are ordered by `created_at` and `id`. Timestamps keep microsecond precision, so rows created within the same millisecond still keep their insertion order.
- `tasks.owner_id` references `users(id)` with `ON DELETE CASCADE`. The `user.deleted` event still removes a user's tasks.
- `status` accepts `todo`, `in_progress` and `done` (default `todo`). `priority` accepts `low`, `medium` and `high`.
- `email` and `username` are matched without regard to letter case.
- `due` is stored as a timestamp with time zone and returned as an ISO 8601 UTC string, for example `2026-12-08T10:00:00.000Z`.

## API

All routes are under `/api/v1`. Routes other than sign up and sign in require the `taskly_token` cookie.

| Method | Path                | Purpose                                         |
| ------ | ------------------- | ----------------------------------------------- |
| POST   | `/auth/signup`      | Register and sign in                            |
| POST   | `/auth/signin`      | Sign in                                         |
| GET    | `/auth/signout`     | Sign out                                        |
| GET    | `/users`            | List users, filter with `username` and `email`  |
| GET    | `/users/:id`        | Get a user                                      |
| PATCH  | `/users/update/:id` | Update own profile                              |
| POST   | `/users/upload/:id` | Upload own avatar (`avatar` field, 2 MB)        |
| DELETE | `/users/delete/:id` | Delete own account                              |
| POST   | `/tasks/create`     | Create a task                                   |
| GET    | `/tasks/user/:id`   | List own tasks with `status`, `orderBy`, `page` |
| GET    | `/tasks/:id`        | Get a task                                      |
| PATCH  | `/tasks/:id`        | Update a task                                   |
| DELETE | `/tasks/:id`        | Delete a task                                   |

Errors use `{ "status": 400, "message": "..." }`. Unknown routes answer `404 { "message": "Not found" }`. JSON bodies are limited to 100 KB.

## Testing

| Layer       | Command                | What it proves                                         |
| ----------- | ---------------------- | ------------------------------------------------------ |
| Unit        | `make test/unit`       | Domain, use cases and adapters, no database            |
| Integration | `make test/integration`| Repositories and HTTP modules on a real PostgreSQL     |
| E2E         | `make test/e2e`        | Built binaries run as real processes                   |
| Coverage    | `make test/coverage`   | Unit and integration tests with coverage               |

Integration and e2e tests need a PostgreSQL server where the user can create databases. They read `TEST_DATABASE_URL` and default to the development database from `make docker/dev-up`. Every test run creates its own temporary database, runs the migrations and drops it afterwards. Integration and e2e files use the `integration` and `e2e` build tags, so a plain `go test ./...` never touches a database. Other checks: `make fmt-check`, `make vet`, `make tidy-check`.

```sh
TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres make test
```

## CI

`.github/workflows/ci.yml` runs these jobs:

1. **Commitlint**: commit message linting on pull requests, configured in `.commitlintrc.json`.
2. **Quality**: `gofmt`, `go vet` and a `go mod tidy` check.
3. **Tests**: unit (with the race detector), integration and end-to-end suites against a PostgreSQL 18 service.
4. **Docker**: builds the image, starts the Compose stack, then checks `/`, signs up a user and runs the seed.

## Limits

- The API needs a running PostgreSQL.
- Uploaded avatars are stored on local disk and served from `/images/users`. The Compose stack keeps them in the `taskly-avatars` volume.
- The upload type check trusts the `Content-Type` declared by the client, like the Express version did.
- Events are delivered in-process, so the app runs as a single instance.
- Data from the former MongoDB version is not imported. Identifiers changed from ObjectId to UUID, so old sessions are invalid and users sign in again.

## Migrated from Express

The Go version keeps routes, status codes, response shapes, cookie name and attributes, validation messages, pagination and the database schema. Existing databases and sessions created by the Express version keep working, because the migrations and the JWT format are unchanged.

Differences to be aware of:

- `due` accepts ISO 8601 and a few common layouts (RFC 1123, `YYYY-MM-DD HH:MM:SS`). It does not accept every free-form string that JavaScript's `Date.parse` tolerates. Values without a time zone are read as UTC.
- Unmatched methods answer `404 Not found` instead of Express's default behaviour, and a malformed multipart body answers `400` instead of `500`.
- JWTs without an `exp` claim are rejected. The server always issued tokens with one.
- Log files rotate daily and at 20 MB, are gzipped and are kept for 14 days. The format is JSON from `log/slog`.

## References

- [Echo](https://echo.labstack.com)
- [PostgreSQL](https://www.postgresql.org/docs/18/)
- [pgx](https://github.com/jackc/pgx)
- [JSON Web Tokens](https://jwt.io)
- [log/slog](https://pkg.go.dev/log/slog)
- [Docker multi-stage builds](https://docs.docker.com/build/building/multi-stage/)
