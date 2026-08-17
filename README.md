# GinBlade

[中文文档](./README.zh-CN.md) · [Architecture](./ARCHITECTURE.md)

[![CI](https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg)](https://github.com/arixbit/ginblade/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/arixbit/ginblade)](https://goreportcard.com/report/github.com/arixbit/ginblade)
[![Version](https://img.shields.io/github/v/tag/arixbit/ginblade?label=version)](https://github.com/arixbit/ginblade/tags)
[![Stars](https://img.shields.io/github/stars/arixbit/ginblade)](https://github.com/arixbit/ginblade)
[![Forks](https://img.shields.io/github/forks/arixbit/ginblade)](https://github.com/arixbit/ginblade/fork)
[![Open Issues](https://img.shields.io/github/issues/arixbit/ginblade)](https://github.com/arixbit/ginblade/issues)
[![Last Commit](https://img.shields.io/github/last-commit/arixbit/ginblade)](https://github.com/arixbit/ginblade/commits)
[![Dependabot](https://img.shields.io/badge/Dependabot-enabled-0366d6)](https://github.com/arixbit/ginblade/security/dependabot)
[![codecov](https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg)](https://codecov.io/gh/arixbit/ginblade)

An opinionated, runnable Go backend skeleton for services that need clear
boundaries, independently deployable processes, and a delivery path that can
be verified from local development through CI.

## Highlights

- Separate API, Asynq worker, and database migration processes.
- `handler -> service -> repository` application flow with explicit boundaries.
- Hand-written dependency injection and centralized resource lifecycle management.
- Gin, GORM, PostgreSQL, optional Redis and JWT support, and asynchronous tasks.
- Multi-stage, non-root container image and a complete local Docker Compose stack.
- Unit, race, integration, lint, and container smoke verification in CI.

Business modules are intentionally kept minimal. The `Example` flow exists to
show how the layers fit together, not to present a complete product or a
universal architecture.

## Quick Start

Start Postgres, Redis, migrations, the API, and the worker:

```sh
make compose-up
curl http://127.0.0.1:3000/health
```

Override host ports when the defaults are already in use:

```sh
POSTGRES_PORT=55433 REDIS_PORT=56380 API_PORT=53000 make compose-up
```

Stop the stack without deleting its data volumes:

```sh
make compose-down
```

## Structure

- `cmd/api`: HTTP API process.
- `cmd/worker`: Asynq worker process.
- `cmd/migrate`: minimal GORM migration entrypoint for the example table.
- `config`: environment loading and typed configuration values.
- `internal/bootstrap`: process-level resource initialization and lifecycle.
- `internal`: application wiring, routes, middleware, and example layers.
- `pkg`: reusable infrastructure helpers, including generic JWT auth.

## Run Locally

```sh
cp .env.example .env
make migrate
go run ./cmd/api
```

Run the worker when Redis is configured:

```sh
go run ./cmd/worker
```

Run the example migration when Postgres is configured:

```sh
go run ./cmd/migrate
```

## Runtime Dependencies

- The API process requires `POSTGRES`.
- Redis is optional for the API process. When configured, it enables cache and queue publishing.
- The worker process requires `REDIS_ADDR`.
- Postgres is optional for the worker process.
- JWT auth example routes are enabled when `JWT_SECRET` is configured.

## Example API

Issue a sample JWT:

```sh
curl -X POST http://127.0.0.1:3000/api/v1/auth/token \
  -H 'Content-Type: application/json' \
  -d '{"subject":"demo"}'
```

Call the protected example endpoint:

```sh
curl http://127.0.0.1:3000/api/v1/auth/me \
  -H "Authorization: Bearer <access_token>"
```

Publish the sample async task when Redis is configured:

```sh
curl -X POST http://127.0.0.1:3000/api/v1/examples/tasks \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo"}'
```

## Startup Flow

```mermaid
flowchart TD
    API["cmd/api"] --> CFG["config.LoadEnv + config.Load"]
    CFG --> BOOT["bootstrap.InitRuntime + bootstrap.InitAPI"]
    BOOT --> REG["Registry: DB, Redis, JWT, Queue"]
    REG --> APP["app.NewServer"]
    APP --> ROUTER["router.RegisterRoutes"]
    ROUTER --> HTTP["/health, /api/v1/auth, /api/v1/examples"]

    WORKER["cmd/worker"] --> WCFG["config.LoadEnv + config.Load"]
    WCFG --> WBOOT["bootstrap.InitRuntime + bootstrap.InitWorker"]
    WBOOT --> WREG["Registry: Redis, optional DB, Queue"]
    WREG --> ASYNQ["app.NewWorker + Asynq handlers"]
```

## Deployment Notes

- Swagger is not enabled in this skeleton. Deployment does not require `swag init`.
- If Swagger is added later, generate docs during development or CI build, not at service startup.
- `CORS_ALLOW_ORIGINS` is a comma-separated allow list. Empty means no CORS allow headers.
- Replace `JWT_SECRET` before using the auth example outside local development.
- API business errors use the JSON envelope `code`, `msg`, and `reason`; most API errors are returned with HTTP 200 by convention.
- `/health` uses real HTTP status codes and returns 503 when required dependencies are unavailable.

## Verify

Run the local CI checks (format, module state, vet, race tests, and builds):

```sh
make ci
```

Run real Postgres, Redis cache, and Redis queue integration tests in an
isolated Compose project:

```sh
make integration-up
make test-integration
make integration-down
```

The integration suite has an explicit `integration` build tag and requires
`TEST_POSTGRES_DSN`, `TEST_REDIS_ADDR`, `TEST_REDIS_CACHE_DB`, and
`TEST_REDIS_QUEUE_DB`. `make test-integration` supplies safe local defaults;
CI points the same tests at its Postgres and Redis service containers.
For concurrent worktrees, override `INTEGRATION_PROJECT`,
`INTEGRATION_POSTGRES_PORT`, and `INTEGRATION_REDIS_PORT` with unique values.

Other useful targets are listed by:

```sh
make help
```
