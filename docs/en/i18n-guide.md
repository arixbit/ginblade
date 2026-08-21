# Internationalization (i18n) Guide

GinBlade does not ship with built-in internationalization. Every user-facing
message is hard-coded English and identical for all requests. This document is
for developers who want the API to return messages in the requester's language,
and shows how to add that without breaking the existing layering or the API
contract.

## Current State

- `pkg/response/response.go` — `messageFor(reason)` is an English-only `switch`
  keyed by the errcode reason.
- `pkg/validator/validator.go` — `HandleValidatorError` is also hard-coded
  (`%s is required`, etc.); it is the exit point for field-validation messages.
- `internal/errcode` — `Error` holds only `code` + `reason`, with no locale
  dimension.
- No middleware reads the request language.

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

## Integration Steps

The snippets below show the recommended shape, not complete drop-in code.

### 1. Add a `pkg/i18n` package

Holds the translation table and lookup logic. Keep it a pure function with no
`gin` dependency, so it is trivial to unit-test.

```go
package i18n

type Translator struct {
    defaultLocale string
    messages    map[string]map[string]string // locale -> reason -> text
    fieldMsgs   map[string]map[string]string // locale -> validator tag -> template
}

func New(defaultLocale string, messages, fieldMsgs map[string]map[string]string) *Translator { /* ... */ }

// Translate resolves an errcode reason to localized copy.
// Missing locale falls back to default; missing key falls back to the reason itself.
func (t *Translator) Translate(locale, reason string) string { /* ... */ }

// TranslateField resolves a validator tag (required/min/max) and fills the field/param.
func (t *Translator) TranslateField(locale, field, tag, param string) string { /* ... */ }
```

Store copy in in-package `map`s rather than external JSON files or a `go-i18n`
dependency. At skeleton scale a map is the most direct option — adding a
language is just adding a map. Upgrade to external files later, when non-engineers
need to maintain copy; the `Translator` interface stays the same.

### 2. Add `internal/middleware/locale.go`

Resolve the request language and stash it on the `gin.Context`, mirroring
`TraceLogger`.

```go
func Locale(defaultLocale string) gin.HandlerFunc {
    return func(c *gin.Context) {
        loc := c.Query("lang") // 1. query (easy to switch during debugging)
        if loc == "" {
            loc = parseAcceptLanguage(c.GetHeader("Accept-Language"), defaultLocale) // 2. header
        }
        if loc == "" {
            loc = defaultLocale // 3. fallback
        }
        c.Set("locale", loc)
        c.Next()
    }
}
```

Resolution priority: `?lang=` query > `Accept-Language` header > configured
default. The query param is convenient for manual switching during development;
`Accept-Language` is the natural source from browsers and clients in production.
For `parseAcceptLanguage`, `golang.org/x/text/language` matches tags with
fallback cleanly, or hand-roll a simple matcher for a small fixed locale set.

Mount it after `TraceLogger` and before business handlers — same per-request
context slot pattern as `trace_id`.

### 3. Modify `pkg/response/response.go`

Have `messageFor` read the locale from the context and look up copy through the
`Translator`:

```go
func ErrorResponse(c *gin.Context, errorCode errcode.Error, tr *i18n.Translator) Response {
    v, _ := c.Get("locale")
    loc, _ := v.(string)
    return Response{
        Code:     errorCode.Code(),
        Reason:   errorCode.Reason(), // unchanged: language-neutral contract
        Message:  tr.Translate(loc, errorCode.Reason()),
        Metadata: buildMetadata(c),
    }
}
```

The `errcode` package, the `Error` struct, and `Reason()` are not touched.

### 4. Modify `pkg/validator/validator.go`

Give `HandleValidatorError` a locale and a `Translator`, routing through
`TranslateField`. `response.BuildValidationErrorResponse` reads the locale off
`c` and passes it through.

### 5. Wire it in bootstrap

- `internal/bootstrap/registry.go` — add an `I18N *i18n.Translator` field to
  `Registry`.
- `internal/bootstrap/api.go` — construct the `Translator` in `InitAPI` and
  store it.
- `internal/server.go` — inject the translator into the response/validator call
  path, and register the `Locale` middleware on the global middleware chain.

### 6. Configuration

Add to `config/types.go`:

```go
type I18nConfig struct {
    DefaultLocale string   // e.g. "en"
    Supported     []string // e.g. ["en","zh"]
}
```

Add `I18n I18nConfig` to `Config`. The default locale comes from config, not a
hard-coded constant.

## Testing

- `pkg/i18n` — table-driven tests covering: hit, locale-missing falls back to
  default, key-missing falls back to the reason itself.
- locale middleware — cover all three sources: query, header, default.
- `ErrorResponse` — given a locale, assert `msg` is the right language and
  `reason` is unchanged.

## Anti-patterns

- **Putting locale into `errcode.Error`.** Letting `reason` vary by language
  breaks the machine-readable contract.
- **Translating inside each handler.** Inconsistent with the `trace_id` pattern;
  handlers should not know how locale was resolved.
- **Reaching for `go-i18n` + JSON files on day one.** Copy volume is small at
  skeleton scale; pulling in file I/O and an extra dependency prematurely adds
  complexity for no gain.
