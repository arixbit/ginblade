package repository

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

func TestTxRunnerInTxReusesTransactionFromContext(t *testing.T) {
	base := newDryRunDB(t, nil)
	tx := newDryRunDB(t, nil)
	runner := NewTxRunner(base)

	var sawTx bool
	err := runner.InTx(WithTx(context.Background(), tx), func(ctx context.Context) error {
		sawTx = txFromContext(ctx) != nil
		return nil
	})
	if err != nil {
		t.Fatalf("InTx: %v", err)
	}
	if !sawTx {
		t.Fatal("expected callback to run with the existing transaction in context")
	}
}

func TestTxRunnerInTxPropagatesCallbackError(t *testing.T) {
	runner := NewTxRunner(newDryRunDB(t, nil))
	want := errors.New("callback failed")

	// Run inside an existing transaction context so InTx takes the
	// reuse path and never opens a real database connection.
	ctx := WithTx(context.Background(), new(gorm.DB))
	err := runner.InTx(ctx, func(context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected callback error to propagate, got %v", err)
	}
}

func TestTxRunnerNilDBReturnsError(t *testing.T) {
	runner := NewTxRunner(nil)
	err := runner.InTx(context.Background(), func(context.Context) error { return nil })
	if err == nil {
		t.Fatal("expected error for nil db")
	}
}
