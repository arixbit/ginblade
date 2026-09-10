package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"
)

type txKey struct{}

var (
	errNilDB   = errors.New("repository: database connection is required")
	errNilTxFn = errors.New("repository: transaction callback is required")
)

// WithTx returns a context carrying the active GORM transaction.
func WithTx(ctx context.Context, tx *gorm.DB) context.Context {
	return context.WithValue(normalizeContext(ctx), txKey{}, tx)
}

// InTx executes fn inside a transaction, reusing an existing transaction in ctx.
func InTx(ctx context.Context, db *gorm.DB, fn func(context.Context) error) error {
	if fn == nil {
		return errNilTxFn
	}
	if tx := txFromContext(ctx); tx != nil {
		return fn(normalizeContext(ctx))
	}
	if db == nil {
		return errNilDB
	}

	baseCtx := normalizeContext(ctx)
	return db.WithContext(baseCtx).Transaction(func(tx *gorm.DB) error {
		return fn(WithTx(baseCtx, tx))
	})
}

func dbFromContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx := txFromContext(ctx); tx != nil {
		return tx
	}
	return db
}

func txFromContext(ctx context.Context) *gorm.DB {
	if ctx == nil {
		return nil
	}
	tx, _ := ctx.Value(txKey{}).(*gorm.DB)
	return tx
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}
