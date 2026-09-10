# Project Overview

[中文文档](./PROJECT-OVERVIEW.zh-CN.md)

A single-page map of GinBlade: what each part is, how the pieces fit, where to
look for a given capability, and which decisions are deliberate rather than
accidental. Use it as the entry point before
[ARCHITECTURE.md](./ARCHITECTURE.md) (design rationale),
[Transactions](docs/en/transactions.md) (how to make several writes atomic),
and the [Quick Start](docs/en/quickstart.md) (copy-paste recipes).

## What This Is

`github.com/arixbit/ginblade` is an opinionated, runnable Go backend skeleton
for services that need clear boundaries, independently deployable processes,
and a delivery path that can be verified from local development through CI.

It is **not** a product and not a universal architecture. Business modules are
deliberately minimal: they exist to show how the layers fit together.

## At a Glance

| | |
|---|---|
| Module | `github.com/arixbit/ginblade` |
| Language | Go 1.26 (toolchain go1.26.5) |
| Packages | 22 |
| Go files | 76 (30 of them tests) |
| Lines of Go | ~6.7k |

### Stack

| Concern | Choice |
|---|---|
| HTTP | Gin v1.10 |
| ORM / database | GORM v1.30 + PostgreSQL (pgx v5) |
| Async tasks | Asynq v0.26 (Redis-backed) |
| Cache | go-redis v9 |
| Logging | zap (JSON / console, carries `trace_id`) |
| Auth | golang-jwt/v5 (HS256) |
| Validation | go-playground/validator v10 |
| Config | environment variables via godotenv |
| Container | multi-stage Dockerfile, non-root `app` user |
| Orchestration | Docker Compose (Postgres 17, Redis 7, migrate, api, worker) |

## Process Model

Three independent binaries, each started by the same sequence:
`config.LoadEnv → config.Load → bootstrap.InitRuntime → bootstrap.Init<Process>
→ app.New<Server|Worker> → run → signal-based graceful shutdown`.

| Process | Requires | Optional | Role |
|---|---|---|---|
| `cmd/api` | Postgres | Redis, JWT | HTTP server (Gin) |
| `cmd/worker` | Redis | Postgres | Asynq task consumer |
| `cmd/migrate` | Postgres | — | GORM `AutoMigrate`, then exits |

## Layering

```
HTTP shell (Gin-coupled)
  handler      bind/validate the request, call the service, write the response
  middleware   trace, recovery, timeout, CORS, auth, rate limit
  router       route registration (skips unconfigured optional modules)
───────────────────────────────────────────────────────────────
Application core (framework-free)
  service      business rules; declares the interfaces it consumes
  repository   persistence (GORM); context-scoped transactions
  model        GORM table models
  task         async task types and payloads
  worker       Asynq server, handlers, trace middleware
  taskqueue    queue boundary wrapper
  errcode      business error codes
───────────────────────────────────────────────────────────────
Infrastructure (pkg/, reusable)
  auth · cache · database · log · response · validator
```

Rules the code actually follows:

- **`handler` never touches the database or the queue.** It calls the service
  and maps errors into the response envelope.
- **The service declares the interfaces it consumes** — `ExampleRepository`,
  `ExampleQueue`, `WalletRepository`, `WalletCache`, `TransactionRunner`.
  Concrete implementations are injected by `bootstrap`, which is what makes the
  service layer unit-testable without a database.
- **`repository` knows GORM and nothing else.** Transactions travel through the
  context: `InTx` opens one (or joins an existing one), and `dbFromContext`
  transparently uses the active transaction.
- **`pkg/` stays reusable.** Nothing under `pkg/` imports `internal/`.

## Reference Flows

Two flows ship side by side. `Example` is the minimum shape a module needs;
`Wallet` is that same shape with the two harder patterns filled in. They are
kept separate rather than merged so each stays readable.

| Capability | Flow | Where |
|---|---|---|
| `handler → service → repository` path | `Example` | `internal/service/example.go` |
| Consumer-defined interfaces + injection | `Example` | `internal/service/example.go` |
| Async task publishing with `trace_id` in the payload | `Example` | `internal/service/example.go`, `internal/task/example.go` |
| Multi-row transaction orchestrated by the service | `Wallet` | `internal/service/wallet.go` |
| Context-propagated transactions | `Wallet` | `internal/repository/tx.go`, `internal/repository/wallet.go` |
| Repository sentinel errors mapped onto `errcode` | `Wallet` | `internal/service/wallet.go` |
| Cache-aside reads with versioned invalidation | `Wallet` | `internal/service/wallet.go` |

### Transactions

The contract has two halves:

- **The service opens the transaction** by calling `tx.InTx`, a
  `service.TransactionRunner`. The concrete `repository.TxRunner` is built from
  the `*gorm.DB` at the composition root and injected in `internal/server.go`,
  so the service never imports GORM.
- **The repository joins it.** Every repository method reads its handle from
  `dbFromContext(ctx, r.db)`, which returns the transaction carried in the
  context if there is one, and the plain connection otherwise.

`WalletRepository.Debit` puts the balance check in the SQL `WHERE` clause
(`id = ? AND balance >= ?`), which makes check-and-update atomic so two
concurrent transfers cannot overdraw the same wallet. Domain failures surface
as sentinel errors (`ErrInsufficientBalance`, `ErrWalletNotFound`) that the
service translates into `errcode` values, keeping HTTP-ish semantics out of the
persistence layer.

### Caching

`WalletService.List` reads through a service-defined `WalletCache` interface
that `internal/server.go` adapts from `*cache.Client`, returning `nil` when
Redis is not configured. Invalidation is versioned: writes bump
`wallet:list:version`, every list key embeds that version, and one `Set`
therefore invalidates every cached list without scanning or deleting keys.

## HTTP Surface

| Method | Path | Notes |
|---|---|---|
| GET | `/health` | Real HTTP status codes; 503 when a required dependency is down |
| POST | `/api/v1/auth/token` | Registered only when `JWT_SECRET` is set |
| GET | `/api/v1/auth/me` | Bearer auth; only when `JWT_SECRET` is set |
| GET | `/api/v1/examples` | Paginated `?limit=&offset=` |
| POST | `/api/v1/examples` | `{"name":"..."}` |
| POST | `/api/v1/examples/tasks` | Publishes an async task; needs Redis |
| GET | `/api/v1/wallets` | Cache-aside when Redis is configured |
| POST | `/api/v1/wallets` | `{"name":"alice","balance":100}` |
| GET | `/api/v1/wallets/:id` | `NOT_FOUND` when absent |
| POST | `/api/v1/wallets/transfers` | `{"from_id":1,"to_id":2,"amount":50}` |

Middleware chain: `TraceLogger → Recovery → Timeout → CORS → RateLimit`
(rate limit only when `RATE_LIMIT_PER_MINUTE > 0`).

### Response Envelope

```json
{ "code": 0, "msg": "success", "data": {...}, "metadata": { "trace_id": "..." } }
```

- Success: `code = 0`, HTTP 200.
- Business errors: `{ code, msg, reason, metadata }` returned with **HTTP 200 by
  convention**; the envelope's `code` is the source of truth. `/health` is the
  deliberate exception and uses real status codes.

| code | reason | meaning |
|---|---|---|
| 1001 | `INVALID_PARAMS` | Request validation failed |
| 1002 | `UNAUTHORIZED` | Missing or invalid credentials |
| 1003 | `PERMISSION_DENIED` | Authenticated but not allowed |
| 1004 | `TOO_MANY_REQUESTS` | Rate limited |
| 1005 | `REQUEST_TIMEOUT` | Request deadline exceeded |
| 2001 | `NOT_FOUND` | Resource does not exist |
| 2002 | `INSUFFICIENT_BALANCE` | Transfer would overdraw the source wallet |
| 9001 | `INTERNAL_ERROR` | Unexpected server-side error |
| 9002 | `DATABASE_ERROR` | Persistence failure |
| 9003 | `QUEUE_UNAVAILABLE` | Queue not configured |
| 9004 | `QUEUE_ERROR` | Task publishing failed |

## Async Tasks

- **Queues:** `critical:6 / default:3 / low:1`, concurrency 10.
- **Retry:** `MaxRetry(5)` with exponential backoff `5s × 2^n`, capped at 1h;
  failures are recorded by an `ErrorHandler`.
- **Tracing:** the publisher writes `trace_id` into the payload and the worker's
  `TraceMiddleware` restores it (or generates a task-scoped one), logging task
  id, queue name, and retry count.

## Configuration

All configuration comes from environment variables, read once by
`config.Load()` with defaults. `config.LoadEnv` loads `cmd/<process>/.env` and
falls back to the repository-root `.env`.

`config/types.go` groups values into `Server`, `Postgres`, `Redis`, `Auth`,
`Cors`, `Log`, and `RateLimit`. **Redis and JWT are optional**: when they are
unconfigured the related routes are not registered, `/health` reports
`not_configured`, cache reads fall back to the database, and everything else
keeps working. See `.env.example` for the full list.

## Engineering

| Target | What it does |
|---|---|
| `make ci` | `fmt-check + tidy-check + verify + vet + test-race + build` |
| `make test` / `make test-race` | Unit tests, optionally under the race detector |
| `make integration-up` / `test-integration` / `integration-down` | Real Postgres + Redis integration tests in an isolated Compose project |
| `make compose-up` / `compose-down` / `compose-logs` | Full local stack |
| `make build` | Builds `api`, `worker`, `migrate` into `bin/` |
| `make lint` | golangci-lint |

Integration tests carry the `integration` build tag and need
`TEST_POSTGRES_DSN`, `TEST_REDIS_ADDR`, `TEST_REDIS_CACHE_DB`, and
`TEST_REDIS_QUEUE_DB`; the Makefile supplies safe local defaults, and
`INTEGRATION_PROJECT` / `INTEGRATION_POSTGRES_PORT` / `INTEGRATION_REDIS_PORT`
let concurrent worktrees avoid collisions.

### Testing Strategy

Three layers, each proving something the others cannot:

| Layer | Mechanism | Proves |
|---|---|---|
| Service | Hand-written mock repository / cache / `TransactionRunner` | Business rules, error mapping, and call ordering run without a database |
| Repository | GORM **DryRun** harness (`internal/repository/example_test.go`) with statement capture | Generated SQL — including the wallet's `balance >= ?` overdraft guard — without a database |
| Integration | Real Postgres in an isolated schema + real Redis (`tests/integration/`) | Actual commit/rollback atomicity, `ErrRecordNotFound` translation, and cache/queue behaviour |

`tests/integration/wallet_test.go` is the reference for the third layer: it
asserts that a transfer commits both legs plus the audit row, that a forced
failure rolls all three back, and that an overdraw is refused with the source
balance untouched.

### CI

| Job | Purpose |
|---|---|
| `test` | Unit checks, integration tests against service containers, coverage upload to Codecov |
| `lint` | golangci-lint v2 (`--build-tags=integration`) |
| `vulncheck` | `govulncheck`; **reports but does not block** |
| `container` | Validates Compose, builds the image, asserts the non-root user, boots the stack, and smoke-tests health → create → enqueue → worker-consumed |
| `release` | On a `v*` tag, GoReleaser publishes linux/darwin × amd64/arm64 binaries |

`.github/workflows/docs.yml` builds the mkdocs site strictly and deploys it to
GitHub Pages.

### Documentation Set

| File | Purpose |
|---|---|
| `README.md` / `README.zh-CN.md` | Orientation, configuration, API surface |
| `ARCHITECTURE.md` / `ARCHITECTURE.zh-CN.md` | Design rationale, layering rules, lifecycles |
| `I18N.md` / `I18N.zh-CN.md` | How to add locales to a service built from this skeleton |
| `PROJECT-OVERVIEW.md` / `PROJECT-OVERVIEW.zh-CN.md` | This document |
| `docs/en/`, `docs/zh/` | mkdocs-material site (index, quickstart, transactions, architecture, i18n guide) |

`docs/en/architecture.md` and `docs/zh/architecture.md` are **copies** of the
root `ARCHITECTURE*.md` files. When you edit one, copy it across — otherwise the
published site goes stale.

## Conventions Worth Knowing

- **Business errors use HTTP 200.** Deliberate, so gateways can handle errors
  uniformly. Change it in `pkg/response` if your team prefers real status codes.
- **`vulncheck` does not block merges.** Standard-library findings depend on a
  toolchain bump, so the job reports rather than fails. Third-party
  vulnerabilities should still be driven to zero.
- **New `internal/` code is framework-free below `handler`.** Only
  `internal/handler`, `internal/middleware`, `internal/router`,
  `internal/server.go`, and `pkg/response` reference Gin. That is what keeps
  swapping the HTTP shell cheap.
- **New models must be registered** in the `AutoMigrate` call in
  `cmd/migrate/main.go`.
