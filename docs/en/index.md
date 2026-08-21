# GinBlade

<div class="gb-hero" markdown>

<div class="gb-badges">
  <img src="https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg" alt="CI">
  <img src="https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go" alt="Go">
  <img src="https://img.shields.io/github/stars/arixbit/ginblade" alt="Stars">
  <img src="https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg" alt="codecov">
</div>

# Build services with *clear boundaries* { }

An opinionated, runnable Go backend skeleton for services that need
explicit layering, independently deployable processes, and a delivery
path verifiable from local development through CI.

<div class="gb-btns">
  <a class="gb-btn gb-btn--primary" href="https://github.com/arixbit/ginblade">★ Star on GitHub</a>
  <a class="gb-btn gb-btn--ghost" href="#quick-start">→ Quick Start</a>
</div>

</div>

---

<div class="gb-section-label">Highlights</div>

## Why GinBlade

<div class="gb-grid">

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Separate processes

API, Asynq worker, and migration are independent binaries. Deploy,
scale, and release each on its own cadence.
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Explicit layering

Application flow is `handler -> service -> repository`. Dependencies
point inward; outer layers depend on interfaces they call.
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Hand-written DI

No DI framework. Resources assembled in `bootstrap`, passed down
explicitly as structs. The dependency graph is visible in one place.
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Optional infrastructure

Redis and JWT are optional. When not configured, routes are skipped,
health reports `not_configured`, the rest keeps working.
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Verified delivery

Unit, race, integration, lint, and container smoke tests in CI.
Multi-stage non-root image + complete Compose stack.
</div>

<div class="gb-card" markdown>
### <span class="gb-dot"></span> Framework-swappable

Only the HTTP shell depends on Gin. Everything below `handler` is
plain Go — swap the router without touching business logic.
</div>

</div>

---

<div class="gb-section-label">Quick Start</div>

## Up and running in 30 seconds

```sh
make compose-up
curl http://127.0.0.1:3000/health
```

Stop the stack without deleting data:

```sh
make compose-down
```

Or run locally without Docker:

```sh
cp .env.example .env
make migrate
go run ./cmd/api
```

---

<div class="gb-section-label">Structure</div>

## Project layout

| Path | Purpose |
|------|---------|
| `cmd/api` | HTTP API process |
| `cmd/worker` | Asynq worker process |
| `cmd/migrate` | GORM migration entrypoint |
| `config` | Environment loading and typed configuration |
| `internal/bootstrap` | Process-level resource initialization |
| `internal` | Application wiring, routes, middleware, layers |
| `pkg` | Reusable infrastructure helpers (JWT, cache, log, …) |

---

<div class="gb-section-label">Next Steps</div>

## Dive deeper

<div class="gb-next">
  <a href="architecture.md"><div class="gb-next-label">📖 Read</div><div class="gb-next-title">Architecture — process model, layering, lifecycles</div></a>
  <a href="i18n-guide.md"><div class="gb-next-label">🌐 Guide</div><div class="gb-next-title">i18n — add request-language-aware messages</div></a>
  <a href="https://github.com/arixbit/ginblade"><div class="gb-next-label">⚡ Source</div><div class="gb-next-title">GitHub — clone, star, contribute</div></a>
</div>
