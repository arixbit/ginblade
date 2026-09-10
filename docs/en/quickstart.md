# Quick Start

This page answers the five questions you will have right after cloning the
repository: where to write a route, where to define a table, where to write
business logic, how to use transactions, and how to use the cache. Every
answer points to a real file with code you can copy.

## Where Things Live

| Path | What goes there |
|------|-----------------|
| `cmd/api/main.go` | HTTP server entrypoint |
| `internal/router/` | Route definitions |
| `internal/handler/` | HTTP request handling: bind params, call service, write response |
| `internal/service/` | Business logic |
| `internal/repository/` | Database operations (GORM) |
| `internal/model/` | Table structures (GORM models) |
| `internal/errcode/` | Business error codes |
| `internal/task/`, `internal/worker/` | Async task types and their handlers |
| `pkg/response/` | Unified response envelope |

The flow of a request: `router → handler → service → repository → Postgres`.

Two reference flows ship with the skeleton. `Example` shows the baseline
`handler → service → repository` path plus async task publishing.
`Wallet` builds on it and adds the two things `Example` does not cover:
multi-row transactions and cache-aside reads.

## 1. Where to Write a Route

File: `internal/router/router.go`

```go
func registerExampleRoutes(r *gin.RouterGroup, deps Dependencies) {
	if deps.Example == nil {
		return
	}

	examples := r.Group("/examples")
	examples.GET("", deps.Example.List)
	examples.POST("", deps.Example.Create)
	examples.POST("/tasks", deps.Example.EnqueueTask)
}
```

Add a line to an existing group, or create a new group and call it from
`RegisterRoutes`.

## 2. Where to Define a Table

File: `internal/model/example.go`

```go
type Example struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"updated_at"`
}

func (Example) TableName() string {
	return "examples"
}
```

The table is created by `cmd/migrate` via GORM `AutoMigrate`. Add new models to
that call — the wallet example registers both `model.Wallet` and
`model.TransferRecord` there.

## 3. Where to Write Business Logic

Two places, in this order:

**Service** (business rules) — `internal/service/example.go`:

```go
type ExampleService struct {
	repo ExampleRepository
}

func NewExampleService(repo ExampleRepository) *ExampleService {
	return &ExampleService{repo: repo}
}

func (s *ExampleService) Create(ctx context.Context, req *CreateExampleReq) (*model.Example, error) {
	example := model.Example{Name: req.Name}
	if err := s.repo.Create(ctx, &example); err != nil {
		return nil, errcode.DatabaseError
	}
	return &example, nil
}
```

**Handler** (bind the request, call the service, write the response) —
`internal/handler/example.go`:

```go
func (h *ExampleHandler) Create(c *gin.Context) {
	var req service.CreateExampleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.WriteValidationError(c, err)
		return
	}

	example, err := h.svc.Create(c.Request.Context(), &req)
	if err != nil {
		response.WriteError(c, err)
		return
	}

	response.WriteSuccess(c, example)
}
```

Return `errcode.Xxx` from services; the handler turns it into the response
envelope. Request/response structs with `binding:` tags live next to the
service method that uses them.

## 4. How to Use Transactions

Use a transaction when one business operation must change several rows and all
of them have to succeed or fail together — a wallet transfer is the classic
case.

The short version: inject `repository.TxRunner` as a
`service.TransactionRunner`, wrap the multi-row work in its `InTx` callback,
and pass the callback's `txCtx` to every repository call.

```go
err := s.tx.InTx(ctx, func(txCtx context.Context) error {
	if err := s.repo.Debit(txCtx, req.FromID, req.Amount); err != nil {
		return err
	}
	return s.repo.Credit(txCtx, req.ToID, req.Amount)
})
```

Returning `nil` commits; returning an error rolls back. The repository methods
need no changes, because each one resolves its handle through `dbFromContext`,
which transparently uses the transaction carried in the context.

→ **[Transactions](transactions.md)** has the full pattern: the three rules for
the callback, why repositories stay untouched, nested transactions, error
mapping, and the SQL guard that stops concurrent overdrafts.

## 5. How to Use the Cache (Cache-Aside)

Redis is optional: when `REDIS_ADDR` is set, `bootstrap` builds a
`*cache.Client`, and when it is not, the same code path has to keep working.
The wallet list read shows the pattern — hit → serve the cached value; miss →
load from the database and fill the cache.

```go
// internal/service/wallet.go
func (s *WalletService) List(ctx context.Context, req *ListWalletsReq) (*ListWalletsRes, error) {
	key := s.listKey(ctx, req.Limit, req.Offset)
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, key); err == nil && cached != "" {
			var wallets []model.Wallet
			if err := json.Unmarshal([]byte(cached), &wallets); err == nil {
				return &ListWalletsRes{Wallets: wallets}, nil
			}
		}
	}

	wallets, err := s.repo.List(ctx, req.Limit, req.Offset)
	if err != nil {
		return nil, errcode.DatabaseError
	}

	if s.cache != nil {
		if raw, err := json.Marshal(wallets); err == nil {
			_ = s.cache.Set(ctx, key, string(raw), walletCacheTTL)
		}
	}
	return &ListWalletsRes{Wallets: wallets}, nil
}
```

Two things make this safe to copy:

- **The cache is a `nil`-able interface.** The service declares `WalletCache`
  itself, and `internal/server.go` adapts `*cache.Client` to it, returning
  `nil` when Redis is not configured. Every use site is guarded, so the
  service never gains a hard dependency on Redis, and unit tests inject a map
  in place of Redis.
- **Invalidation is versioned.** Writes bump `wallet:list:version`, and every
  list key embeds that version. One `Set` therefore invalidates all cached
  lists at once, with no key scanning or deletion.

## Wiring

`internal/server.go` assembles repository → service → handler. Follow the
existing `Example` and `Wallet` wiring to connect your new pieces, and the
route group from step 1 picks them up.
