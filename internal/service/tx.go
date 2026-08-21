package service

import "context"

// TransactionRunner executes a callback inside a database transaction.
// Services declare this interface; the concrete implementation is provided
// by the repository layer and injected through bootstrap.
type TransactionRunner interface {
	InTx(ctx context.Context, fn func(context.Context) error) error
}
