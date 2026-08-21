# Architecture

This document describes the design of GinBlade: the process model, the
layering rules, dependency management, request/task lifecycles, and how to
extend the skeleton with new business modules. It is intended for anyone who
picks this repository up as the starting point of a real service.

## Design Principles

1. **Separate processes, not one monolith.** The API, the async worker, and
   the migration runner are independent binaries. They can be deployed,
   scaled, and released independently.
2. **Explicit layering.** Application flow is `handler -> service ->
   repository`. Dependencies point inward: outer layers depend on interfaces
   defined by the layer they call.
3. **Hand-written dependency injection.** No DI framework. Resources are
   assembled in `bootstrap` and passed down explicitly as structs. This keeps
   the dependency graph visible in one place and avoids magic.
4. **Centralized lifecycle.** Every shared resource (DB pool, Redis client,
   queue client) is owned by a `bootstrap.Registry` and released in one place
   on shutdown.
5. **Optional infrastructure.** Redis and JWT are optional. When they are not
   configured, the corresponding routes are not registered, the health check
   reports `not_configured`, and the rest of the service keeps working.
6. **Framework is an implementation detail.** Only the HTTP shell depends on
   Gin. Everything below `handler` is plain Go, so the HTTP layer can be
   swapped if the standard library (or another router) ever becomes a better
   fit.

## Process Model

```
                ┌──────────────────────────────────────────────┐
                │                    Registry                  │
                │  Cfg · DBManager · Cache · JWT · Queue       │
                └───────┬──────────────┬──────────┬────────────┘
                        │              │          │
        ┌───────────────▼───┐   ┌──────▼───────┐  │
        │  cmd/api          │   │ cmd/worker   │  │
        │  HTTP server      │   │ Asynq worker │  │
        │  (Gin)            │   │              │  │
        └───────────────┬───┘   └──────┬───────┘  │
                        │              │          │
                        ▼              ▼          ▼
                   ┌──────────┐  ┌──────────┐  ┌──────────────┐
                   │ Postgres │  │  Redis   │  │  migrations  │
                   │          │  │ (queue + │  │  (cmd/migrate)│
                   └──────────┘  │  cache)  │  └──────────────┘
                                 └──────────┘
```

- **`cmd/api`** — serves HTTP. Requires Postgres; Redis and JWT are optional.
- **`cmd/worker`** — consumes Asynq tasks. Requires Redis; Postgres is
  optional (only needed by handlers that touch the database).
- **`cmd/migrate`** — runs GORM `AutoMigrate` for the example table. In the
  Compose stack it runs once and exits before `api`/`worker` start.

Each process follows the same startup sequence:

```
config.LoadEnv → config.Load → bootstrap.InitRuntime → bootstrap.Init<Process>
→ app.New<Server|Worker> → run with signal-based graceful shutdown
```

## Layering

```
HTTP shell (Gin-coupled)
  handler      parse/validate request, call service, write response
  middleware   trace, recovery, timeout, CORS, auth, rate limit
  router       route registration (skips optional modules when unconfigured)
───────────────────────────────────────────────────────────────
Application core (framework-free)
  service      business rules; declares the repository/queue interfaces it needs
  repository   persistence (GORM); context-scoped transactions (InTx/WithTx)
  model        GORM table models
  task         async task types and payloads
  worker       Asynq server, handlers, trace middleware
  taskqueue    queue boundary wrapper
  errcode      business error codes
───────────────────────────────────────────────────────────────
Infrastructure (pkg/, reusable)
  auth · cache · database · log · response · validator
```

Rules:

- **`handler` never touches the database or the queue directly.** It only
  calls the service and maps errors to the response envelope.
- **`service` defines the interfaces it consumes** (`ExampleRepository`,
  `ExampleQueue`). The concrete implementations are injected by `bootstrap`.
  This is the consumer-defines-interface pattern and it is what makes the
  service layer unit-testable without a database.
- **`repository` knows GORM, nothing else.** Transaction support is carried
  through the context: `InTx` starts a transaction (or joins an existing one
  from the context), `dbFromContext` transparently uses the active
  transaction inside repositories.
- **`worker` handlers receive `Deps`** (DB, cache, Redis client, queue) as a
  struct, mirroring the handler-layer injection style.

## Dependency Injection & Lifecycle

`internal/bootstrap` is the composition root:

- `InitAPI` / `InitWorker` build the shared resources and return a
  `*Registry`.
- Optional resources return `nil` when their configuration is absent
  (`initCache` returns nil without Redis addr, `initAuth` without JWT secret).
- `Registry.Close()` releases everything it owns and aggregates errors with
  `errors.Join`, so a partial failure still closes the rest.

The HTTP handlers and the worker deps are then assembled in
`internal/server.go` and `internal/worker.go` respectively.

## HTTP Request Lifecycle

```
client
  │  X-Request-ID (optional)
  ▼
TraceLogger   generate/forward trace id; audit log on completion
Recovery      panic → errcode.InternalError response
Timeout       attach context deadline (REQUEST_TIMEOUT)
CORS          allow-list check, OPTIONS short-circuit
RateLimit     in-memory per-IP token bucket (when enabled)
  ▼
router → handler → service → repository → Postgres
  ▼
response envelope { code, msg, reason, data, metadata{trace_id} }
```

Conventions:

- Success: `code = 0`, HTTP 200.
- Business errors: `{ code, msg, reason, metadata }`, **HTTP 200 by
  convention** (the code in the envelope is the source of truth).
- `/health` is the exception: it uses real HTTP status codes and returns 503
  when a required dependency is unavailable.

## Async Task Lifecycle

1. **Publish** — `service` builds a task with
   `task.NewExampleTask(name, traceID)` and calls `queue.Enqueue(ctx, task)`.
   The `trace_id` from the request context is embedded in the payload.
2. **Consume** — the worker's `ServeMux` routes by task type to the handler.
   A `TraceMiddleware` restores the `trace_id` from the payload (or creates
   one scoped to the task), and logs start/finish/failure events with task
   id, queue name, and retry count.
3. **Retry** — `MaxRetry(5)` with exponential backoff `5s × 2^n` capped at
   1h; an `ErrorHandler` records failures.

Three queues with priorities: `critical:6 / default:3 / low:1`, concurrency
10.

## Configuration

All configuration comes from environment variables, read once in
`config.Load()` with sensible defaults. `config.LoadEnv` loads
`cmd/<process>/.env` and falls back to the repository-root `.env`. See
`README.md` for the full variable table.

## Extending: Add a New Business Module

Copy the `Example` flow. For a synchronous feature:

1. `internal/model/` — GORM model.
2. `internal/repository/` — repository with the methods your service needs;
   use `InTx`/`dbFromContext` if the flow needs transactions.
3. `internal/service/` — business logic; declare the repository interface
   here; expose typed request/response structs with validation tags.
4. `internal/handler/` — bind/validate, call service, write response.
5. `internal/router/` — register routes.
6. `internal/server.go` — wire repository → service → handler.

For an asynchronous feature, additionally:

7. `internal/task/` — task type constant, payload struct, constructor.
8. `internal/worker/handler.go` — the handler; register it in
   `RegisterHandlers`.
9. `internal/service/` — enqueue the task (respect `queue.Available()`).

## Framework Coupling (why swapping Gin is cheap)

The framework boundary is deliberate. These packages reference `gin`:

```
internal/handler   internal/middleware   internal/router
internal/server.go pkg/response
```

These do **not**:

```
internal/service  internal/repository  internal/model
internal/task     internal/taskqueue   internal/worker
internal/bootstrap
pkg/auth  pkg/cache  pkg/database  pkg/log  pkg/validator
```

Business rules, persistence, task processing, and infrastructure helpers are
framework-free. If the standard library router (Go 1.22+ `http.ServeMux`
already supports method matching and path wildcards) or another framework
ever becomes the better choice, migrating means rewriting the HTTP shell —
handlers, middleware, router, and `pkg/response` — while the application core
stays untouched.
