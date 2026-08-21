---
hide:
  - navigation
  - toc
---

<div class="gb-hero" markdown>

<div class="gb-badges">
  <img src="https://github.com/arixbit/ginblade/actions/workflows/ci.yml/badge.svg" alt="CI">
  <img src="https://img.shields.io/github/go-mod/go-version/arixbit/ginblade?label=Go" alt="Go">
  <img src="https://img.shields.io/github/stars/arixbit/ginblade" alt="Stars">
  <img src="https://codecov.io/gh/arixbit/ginblade/branch/main/graph/badge.svg" alt="codecov">
</div>

<h1>Build services with <em>clear boundaries</em></h1>

An opinionated, runnable Go backend skeleton for services that need
explicit layering, independently deployable processes, and a delivery
path verifiable from local development through CI.

<div class="gb-btns">
  <a class="gb-btn gb-btn--primary" href="https://github.com/arixbit/ginblade">★ Star on GitHub</a>
  <a class="gb-btn gb-btn--ghost" href="quickstart/">→ Quick Start</a>
</div>

</div>

---

<div class="gb-section-label">Highlights</div>

## Why GinBlade

<div class="gb-grid">
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Separate processes</h3>
    <p>API, Asynq worker, and migration are independent binaries. Deploy, scale, and release each on its own cadence.</p>
  </div>
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Explicit layering</h3>
    <p>Application flow is <code>handler -&gt; service -&gt; repository</code>. Dependencies point inward; outer layers depend on interfaces they call.</p>
  </div>
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Hand-written DI</h3>
    <p>No DI framework. Resources assembled in <code>bootstrap</code>, passed down explicitly as structs. The dependency graph is visible in one place.</p>
  </div>
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Optional infrastructure</h3>
    <p>Redis and JWT are optional. When not configured, routes are skipped, health reports <code>not_configured</code>, the rest keeps working.</p>
  </div>
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Verified delivery</h3>
    <p>Unit, race, integration, lint, and container smoke tests in CI. Multi-stage non-root image + complete Compose stack.</p>
  </div>
  <div class="gb-card">
    <h3><span class="gb-dot"></span> Framework-swappable</h3>
    <p>Only the HTTP shell depends on Gin. Everything below <code>handler</code> is plain Go — swap the router without touching business logic.</p>
  </div>
</div>

---

<div class="gb-section-label">Quick Start</div>

## Start your first feature

<div class="gb-cta">
  <p>From cloning the repo to writing your first route, service method, model, and cross-table transaction — the on-ramp walks through each layer with real code from the skeleton.</p>
  <a class="gb-btn gb-btn--primary" href="quickstart/">Read the Quick Start guide →</a>
</div>

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
