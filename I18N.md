# Internationalization (i18n) Guide

This branch ships working i18n for API messages: `msg` in the response envelope
follows the requester's language, while everything else in the API contract
stays the same. (`main` has no i18n; this branch is the runnable reference.)
This guide explains how it works, where the message files live and why, and how
to add languages and messages.

## Core Principle: locale lives in the message layer, not in errcode

This is the most important rule in this guide. `errcode.Error`'s `reason`
(`INVALID_PARAMS`, etc.) is a **stable machine-readable contract** — clients and
SDKs switch on it to branch on error type. If `reason` changed with language,
every downstream branch would break.

Therefore:

- `reason` stays language-neutral — always `INVALID_PARAMS`.
- `msg` carries the translatable text, resolved per request locale.
- The response JSON shape does not change: `{ code, msg, reason, data, metadata }`.

Separating contract from copy is what lets you add i18n without touching the
existing API surface.

## How It Works

```
request ─▶ Locale middleware ─▶ c.Set("localizer", tr.Localizer(?lang, Accept-Language))
                                        │
handler/middleware error ─▶ pkg/response ─▶ localizer.Error(reason)          ─▶ msg
validation error         ─▶ pkg/validator ─▶ localizer.Validation(tag, ...) ─▶ msg
```

| Piece | Role |
|---|---|
| `internal/locales/` | The catalogs (`en.json`, `zh-CN.json`), embedded with `//go:embed` |
| `pkg/i18n` | Loads catalogs from any `fs.FS`, matches locales, looks up messages; no Gin |
| `internal/middleware/locale.go` | Resolves the request locale and stores a `Localizer` |
| `pkg/response`, `pkg/validator` | Read the `Localizer` to fill `msg` |
| `internal/bootstrap` | `i18n.New(locales.FS, cfg.I18n.DefaultLocale)` → `Registry.I18n` |

`Locale` is mounted right after `TraceLogger`, so errors written by `Recovery`,
`RateLimit`, and `BearerAuth` are localized too. Handlers never see the locale.

## Where the Catalogs Live, and Why

The JSON files sit in `internal/locales/` and are embedded by `locales.go` in
the same directory. Three constraints lead there:

1. **`//go:embed` cannot reach parent directories** (`../locales/*.json` is an
   invalid pattern), so the package that embeds the files must contain them.
2. **Production images contain only binaries.** The `Dockerfile` copies `/out/`
   and GoReleaser archives only `LICENSE` and `README.md`. Embedding means no
   build, deploy, or working-directory changes.
3. **The catalog keys are this app's contract**, so the files belong under
   `internal/` (next to `internal/errcode`), while the reusable mechanism lives
   in `pkg/i18n` — the same split as `pkg/response` + `internal/errcode`.

A root-level `locales/` would need either a Go package at the module root (making
the copy part of the module's public API) or runtime file loading. `pkg/i18n`
takes an `fs.FS`, so if you ever need to change copy without a redeploy, swap
`locales.FS` for `os.DirFS(dir)` in bootstrap; nothing else changes.

## Catalog Format

One file per locale, named with a BCP 47 tag (`en.json`, `zh-CN.json`):

```json
{
  "errors": {
    "INVALID_PARAMS": "请求参数错误",
    "default": "操作失败"
  },
  "validation": {
    "required": "{field} 不能为空",
    "min": "{field} 至少为 {param}",
    "default": "{field} 格式不正确"
  }
}
```

- `errors` is keyed by errcode **reason**, not numeric code: readable, and it
  already is the stable contract.
- `validation` is keyed by validator tag; `{field}` and `{param}` are named
  placeholders, so translations can reorder them freely.
- `default` in each section covers reasons and tags without their own entry.

Lookup order: the key in the request locale → the key in the default locale →
`default` in the request locale → `default` in the default locale → the key
itself. With no `Locale` middleware on the route, `msg` is the bare reason.

## Locale Resolution

Priority: `?lang=` query > `Accept-Language` header > `I18N_DEFAULT_LOCALE`
(default `en`). Matching uses `golang.org/x/text/language`, so `q` weights are
honored, `zh` matches `zh-CN`, `en-GB` matches `en`, and an
unsupported first choice falls through to the next.

Note that `zh-TW` and `zh-HK` also match `zh-CN`, the closest available. To
serve Traditional Chinese, add `zh-TW.json`.

## Common Tasks

- **Add a language** — copy `en.json` to `<tag>.json` in `internal/locales/` and
  translate the values. It is picked up automatically.
- **Add a message for a new errcode** — add the reason under `errors` in
  **every** catalog. Missing it in one file fails CI (see below).
- **Change the default language** — set `I18N_DEFAULT_LOCALE`. Startup fails if
  no catalog matches it.

## Testing

- `pkg/i18n` — table-driven tests on an in-memory `fstest.MapFS`: loading
  errors, locale matching, and every fallback step.
- `internal/locales` — loads the real catalogs and asserts **every file has
  exactly the same keys as `en.json`**. A missing translation fails CI with
  `zh-CN.json is missing [errors.QUEUE_ERROR]`, instead of silently falling back
  in production.
- `internal/middleware` — query, header, and default sources; `reason` never
  changes.
- `internal/server_test.go` — end to end through `NewServer`: a `zh-CN` request
  with an empty body gets `msg: "name 不能为空"`.

## Known Limits

- The success `msg` stays `"success"`.
- Binding errors that are not validation errors (malformed JSON) still return
  Go's error text.
- Field names in validation messages are the lowercased struct field names.
- Changing copy requires a rebuild (see the `os.DirFS` swap above).
- The worker does not localize. If tasks need it, carry the locale in the task
  payload the way `trace_id` is carried.

## Anti-patterns

- **Putting locale into `errcode.Error`.** Letting `reason` vary by language
  breaks the machine-readable contract.
- **Translating inside each handler.** Inconsistent with the `trace_id` pattern;
  handlers should not know how locale was resolved.
- **Reading catalogs from the working directory by default.** The image ships
  only binaries; embed first, and switch to `os.DirFS` only when hot updates are
  a real requirement.
