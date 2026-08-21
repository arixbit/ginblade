# Quick Start

This page answers the four questions you will have right after cloning the
repository: where to write a route, where to define a table, where to write
business logic, and how to use transactions. Every answer points to a real
file with code you can copy.

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

The table is created by `cmd/migrate` via GORM `AutoMigrate`.

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

Use a transaction when one business operation must change several rows and
all of them have to succeed or fail together — a wallet transfer is the
classic case: money leaves one account and lands in another, and you never
want only half of that to happen.

The skeleton provides `repository.TxRunner`, injected into services through
the `service.TransactionRunner` interface. The skeleton's `WalletService`
shows the pattern end to end.

The service holds a `TransactionRunner` and wraps the multi-row work in its
`InTx` callback:

```go
// internal/service/wallet.go
type WalletService struct {
	repo WalletRepository
	tx   TransactionRunner
}

func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) (*TransferRes, error) {
	if req.FromUserID == req.ToUserID {
		return nil, errcode.InvalidParams
	}

	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.AddBalance(txCtx, req.FromUserID, -req.Amount); err != nil {
			return err
		}
		return s.repo.AddBalance(txCtx, req.ToUserID, req.Amount)
	})
	if err != nil {
		return nil, errcode.DatabaseError
	}
	return &TransferRes{Success: true}, nil
}
```

The two `AddBalance` calls are the debit and the credit. Both run inside one
transaction:

- the debit succeeds and the credit fails → the debit is rolled back, the
  transfer returns an error, no money moves;
- both succeed → the transaction commits.

The repository methods are unchanged — each one already goes through
`dbFromContext`, which transparently uses the transaction from the context
when present and the plain connection otherwise:

```go
// internal/repository/wallet.go
func (r *WalletRepository) AddBalance(ctx context.Context, userID uint64, delta int64) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("user_id = ?", userID).
		UpdateColumn("balance", gorm.Expr("balance + ?", delta)).Error
}
```

Nested `InTx` calls join the outer transaction instead of opening a new one,
so composing transactions inside transactions is safe.

The wiring lives in `internal/server.go`: the repository and the `TxRunner`
are both built from the database connection and passed into the service.

```go
walletRepository := repository.NewWalletRepository(db)
walletService := service.NewWalletService(walletRepository, repository.NewTxRunner(db))
```

## Wiring

`internal/server.go` assembles repository → service → handler. Follow the
existing `Example` wiring to connect your new pieces, and the route group from
step 1 picks them up.
