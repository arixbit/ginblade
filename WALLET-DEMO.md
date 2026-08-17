# Wallet Demo Module (branch: examples/wallet)

This branch demonstrates two capabilities the GinBlade skeleton ships but the
`Example` flow does not use:

1. **Transaction orchestration at the service layer** —
   `internal/service/wallet.go` opens a transaction via the injected
   `TxRunner` interface (`repository.InTx` bound to the DB handle at the
   composition root) and coordinates three repository calls inside it:
   debit, credit, and audit record. The repository exposes individual atomic
   operations (`Debit`, `Credit`, `CreateTransferRecord`) that transparently
   join the caller's transaction through `dbFromContext`. This is the pattern
   for cross-table / cross-repository use cases: business orchestration lives
   in the service, persistence primitives live in the repository.
2. **Cache-aside reads** — `internal/service/wallet.go` reads wallet lists
   through Redis when a cache is configured: hit → serve cached; miss → load
   from DB and fill. Writes bump a list version key, which invalidates cached
   lists by switching to a new cache key.

It also shows:

- Extending the error catalog (`errcode.NotFound`, `errcode.InsufficientBalance`)
- Service-layer unit tests with mocked repository and cache
  (`internal/service/wallet_test.go`)

## API

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/wallets?limit=&offset=` | List wallets (cached when Redis is configured) |
| POST | `/api/v1/wallets` | Create wallet `{"name":"alice","balance":100}` |
| POST | `/api/v1/wallets/transfer` | Transfer `{"from_id":1,"to_id":2,"amount":50}` |

## How to use this demo

This branch is intentionally **not merged into main** — main keeps the minimal
`Example` flow as the reference skeleton. To use the wallet module in your own
project, copy these pieces:

```
internal/model/wallet.go          # models
internal/repository/wallet.go     # atomic ops + dbFromContext (join caller tx)
internal/service/wallet.go        # TxRunner + cache-aside orchestration demo
internal/handler/wallet.go        # HTTP handlers
internal/service/wallet_test.go   # mocked tests
```

and wire them in `internal/server.go`, `internal/router/router.go`, and
`cmd/migrate/main.go` (all already wired on this branch).
