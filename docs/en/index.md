# GinBlade

[![CI](https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg)](https://github.com/arixbit/ginblade/actions/workflows/ci.yml)
[![Go](https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go)](https://go.dev/)
[![Go Report Card](https://goreportcard.com/badge/github.com/arixbit/ginblade)](https://goreportcard.com/report/github.com/arixbit/ginblade)
[![Version](https://img.shields.io/github/v/tag/arixbit/ginblade?label=version)](https://github.com/arixbit/ginblade/tags)
[![Stars](https://img.shields.io/github/stars/arixbit/ginblade)](https://github.com/arixbit/ginblade)
[![codecov](https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg)](https://codecov.io/gh/arixbit/ginblade)

---

An opinionated, runnable Go backend skeleton for services that need clear
boundaries, independently deployable processes, and a delivery path that can
be verified from local development through CI.

:octicons-star-24: [Star on GitHub](https://github.com/arixbit/ginblade){ .md-button .md-button--primary }
:octicons-rocket-24: [Quick Start](#quick-start){ .md-button }

---

## Highlights

:material-package-variant-closed: **Separate processes, not one monolith**

:   API, Asynq worker, and database migration are independent binaries.
    Deploy, scale, and release each on its own cadence.

:material-layers-triple: **Explicit layering**

:   Application flow is `handler -> service -> repository`. Dependencies
    point inward; outer layers depend on interfaces defined by the layer
    they call.

:material-hand-coin: **Hand-written dependency injection**

:   No DI framework. Resources are assembled in `bootstrap` and passed down
    explicitly as structs. The dependency graph is visible in one place.

:material-database-cog: **Optional infrastructure**

:   Redis and JWT are optional. When not configured, the corresponding
    routes are not registered, health check reports `not_configured`, and
    the rest of the service keeps working.

:material-test-tube: **Verified delivery path**

:   Unit, race, integration, lint, and container smoke tests run in CI.
    Multi-stage, non-root container image and a complete local Docker
    Compose stack.

---

## Quick Start

Start Postgres, Redis, migrations, the API, and the worker:

```sh
make compose-up
curl http://127.0.0.1:3000/health
```

Stop the stack without deleting its data volumes:

```sh
make compose-down
```

Run locally without Docker:

```sh
cp .env.example .env
make migrate
go run ./cmd/api
```

---

## Structure

| Path | Purpose |
|------|---------|
| `cmd/api` | HTTP API process |
| `cmd/worker` | Asynq worker process |
| `cmd/migrate` | GORM migration entrypoint |
| `config` | Environment loading and typed configuration |
| `internal/bootstrap` | Process-level resource initialization |
| `internal` | Application wiring, routes, middleware, layers |
| `pkg` | Reusable infrastructure helpers (JWT, cache, log, ...) |

---

## Next Steps

- :material-book-open-page-variant: [Architecture](architecture.md) - process model, layering, lifecycles
- :material-translate: [i18n Guide](i18n-guide.md) - how to add request-language-aware messages
- :material-github: [Source on GitHub](https://github.com/arixbit/ginblade)
