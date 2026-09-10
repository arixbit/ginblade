# Transactions

How to make several database writes succeed or fail together.

The skeleton ships exactly one transaction primitive, `repository.TxRunner`,
and the `Wallet` module is the worked example. This page covers when you need a
transaction, the exact code to write, and why the repository layer needs no
changes at all.

## When You Need One

Reach for a transaction when a single request writes to more than one row or
table **and** a partially applied result would be a bug rather than an
inconvenience.

A wallet transfer is the classic case: money leaves one account and lands in
another. If the debit commits and the credit does not, money disappears. Get
interrupted halfway and the books no longer balance.

If your handler only writes a single row, you do not need anything here — GORM
already wraps one `Create`/`Update` in its own transaction.

## The Primitive

`internal/service/tx.go` defines the interface services depend on:

```go
type TransactionRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}
```

`internal/repository/tx.go` supplies the implementation, `TxRunner`, which is
`InTx` bound to a `*gorm.DB`:

```go
func NewTxRunner(db *gorm.DB) *TxRunner
```

`internal/server.go` builds it from the shared database handle and injects it:

```go
walletService := service.NewWalletService(
	repository.NewWalletRepository(db),
	walletCache(reg.Cache),
	repository.NewTxRunner(db),
)
```

Your service holds the interface and never touches GORM:

```go
type WalletService struct {
	repo WalletRepository
	tx   TransactionRunner
}
```

## Writing the Callback

Wrap the multi-row work in `InTx`. Everything inside receives `txCtx`, which
carries the transaction:

```go
func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) (*TransferRes, error) {
	if req.FromID == req.ToID {
		return nil, errcode.InvalidParams
	}

	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.Debit(txCtx, req.FromID, req.Amount); err != nil {
			return err
		}
		if err := s.repo.Credit(txCtx, req.ToID, req.Amount); err != nil {
			return err
		}
		return s.repo.CreateTransferRecord(txCtx, req.FromID, req.ToID, req.Amount)
	})
	if err != nil {
		return nil, errcode.DatabaseError
	}
	return &TransferRes{Success: true}, nil
}
```

Three rules make it correct:

1. **Pass `txCtx` down, never the outer `ctx`.** Handing a repository the outer
   context quietly runs that statement outside the transaction, where it will
   survive the rollback.
2. **`nil` commits, any error rolls back.** You never call `Commit` or
   `Rollback` yourself.
3. **Return on the first failure.** Once a step fails, return immediately
   instead of issuing more writes and hoping to tidy up afterwards.

## Why Repositories Do Not Change

The repository never learns that a transaction exists. Every method resolves
its handle through `dbFromContext`, which returns the transaction carried in
the context when there is one, and the plain connection otherwise:

```go
// internal/repository/tx.go
func dbFromContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return db
}
```

So `Debit` is written exactly the same way whether it is called inside a
transfer or on its own:

```go
func (r *WalletRepository) Debit(ctx context.Context, id uint64, amount int64) error {
	res := dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ? AND balance >= ?", id, amount).
		UpdateColumn("balance", gorm.Expr("balance - ?", amount))
	// ...
}
```

Two consequences worth internalising:

- **Keep repository methods single-purpose.** They are the atoms; the service
  decides which ones travel together.
- **Never reach for `dbFromContext` from a service.** It is unexported for
  exactly that reason — services talk in terms of `TransactionRunner`.

## Nested Transactions Join

Calling `InTx` while a transaction is already active in the context does not
open a second one: the callback runs inside the existing transaction. A service
method can therefore safely call another service method that also uses `InTx`.

```go
s.tx.InTx(ctx, func(txCtx context.Context) error {
	return otherService.DoSomething(txCtx) // also uses InTx internally
})
```

The inner call joins the outer transaction, and a failure in either rolls back
the whole thing.

## Mapping Errors

`InTx` returns whatever the callback returned. Repository failures are sentinel
errors, so a service can branch on them and pick the business code:

```go
if err != nil {
	switch {
	case errors.Is(err, repository.ErrInsufficientBalance):
		return nil, errcode.InsufficientBalance
	case errors.Is(err, repository.ErrWalletNotFound):
		return nil, errcode.NotFound
	default:
		return nil, errcode.DatabaseError
	}
}
```

Use `errors.Is` rather than `==`, because the error may have been wrapped on
the way up.

## Put the Guard in the SQL

A transaction makes a set of writes atomic. It does **not** by itself stop two
concurrent transactions from both acting on a stale balance. `Debit` closes
that gap by moving the check into the `WHERE` clause:

```sql
UPDATE wallets SET balance = balance - $1 WHERE id = $2 AND balance >= $1
```

When the balance is too low the update matches no row, `RowsAffected` comes
back zero, and the repository returns `ErrInsufficientBalance`. The check and
the update are one atomic statement. A separate "read the balance, compare,
then write" would be a race.

## Where to See It Working

| What | Where |
|---|---|
| Service orchestration | `internal/service/wallet.go` |
| The interface services depend on | `internal/service/tx.go` |
| Transaction primitive | `internal/repository/tx.go` |
| Wiring | `internal/server.go` |
| Mocked orchestration tests | `internal/service/wallet_test.go` |
| Generated-SQL tests (GORM dry run) | `internal/repository/wallet_test.go` |
| Real commit/rollback against Postgres | `tests/integration/wallet_test.go` |
