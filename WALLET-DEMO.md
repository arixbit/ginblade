# Wallet Demo Module (branch: examples/wallet)

This branch demonstrates two capabilities the GinBlade skeleton ships but the
`Example` flow does not use:

1. **Transactional writes** — `internal/repository/wallet.go` uses `InTx`
   (from `repository/tx.go`) so a transfer's debit, credit, and audit record
   run in one atomic transaction. The debit uses a guarded UPDATE
   (`WHERE id = ? AND balance >= ?`) so concurrent transfers cannot overdraw
   the source wallet.
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
internal/repository/wallet.go     # InTx transaction demo
internal/service/wallet.go        # cache-aside + service interface demo
internal/handler/wallet.go        # HTTP handlers
internal/service/wallet_test.go   # mocked tests
```

and wire them in `internal/server.go`, `internal/router/router.go`, and
`cmd/migrate/main.go` (all already wired on this branch).
