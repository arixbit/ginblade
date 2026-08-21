package service

import (
	"context"
	"errors"
	"testing"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/model"
)

type mockWalletRepo struct {
	addBalanceFunc func(ctx context.Context, userID uint64, delta int64) error
}

func (m *mockWalletRepo) GetByUserID(ctx context.Context, userID uint64) (*model.Wallet, error) {
	return nil, nil
}

func (m *mockWalletRepo) AddBalance(ctx context.Context, userID uint64, delta int64) error {
	if m.addBalanceFunc == nil {
		return nil
	}
	return m.addBalanceFunc(ctx, userID, delta)
}

type mockTxRunner struct {
	inTxFunc func(ctx context.Context, fn func(context.Context) error) error
}

func (m *mockTxRunner) InTx(ctx context.Context, fn func(context.Context) error) error {
	if m.inTxFunc == nil {
		return fn(ctx)
	}
	return m.inTxFunc(ctx, fn)
}

func TestTransferSuccess(t *testing.T) {
	var debited, credited uint64
	repo := &mockWalletRepo{
		addBalanceFunc: func(_ context.Context, userID uint64, delta int64) error {
			switch delta {
			case -100:
				debited = userID
			case 100:
				credited = userID
			}
			return nil
		},
	}
	svc := NewWalletService(repo, &mockTxRunner{})

	res, err := svc.Transfer(context.Background(), &TransferReq{FromUserID: 1, ToUserID: 2, Amount: 100})
	if err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !res.Success {
		t.Fatal("expected success response")
	}
	if debited != 1 {
		t.Fatalf("expected debit on user 1, got %d", debited)
	}
	if credited != 2 {
		t.Fatalf("expected credit on user 2, got %d", credited)
	}
}

func TestTransferRunsInsideTransaction(t *testing.T) {
	var sawTxCtx bool
	repo := &mockWalletRepo{}
	tx := &mockTxRunner{
		inTxFunc: func(_ context.Context, fn func(context.Context) error) error {
			sawTxCtx = true
			return fn(context.Background())
		},
	}
	svc := NewWalletService(repo, tx)

	if _, err := svc.Transfer(context.Background(), &TransferReq{FromUserID: 1, ToUserID: 2, Amount: 100}); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !sawTxCtx {
		t.Fatal("expected transfer to run inside TransactionRunner.InTx")
	}
}

func TestTransferMapsRepoErrorToDatabaseError(t *testing.T) {
	repo := &mockWalletRepo{
		addBalanceFunc: func(_ context.Context, userID uint64, delta int64) error {
			return errors.New("connection lost")
		},
	}
	svc := NewWalletService(repo, &mockTxRunner{})

	_, err := svc.Transfer(context.Background(), &TransferReq{FromUserID: 1, ToUserID: 2, Amount: 100})
	if err != errcode.DatabaseError {
		t.Fatalf("expected errcode.DatabaseError, got %v", err)
	}
}

func TestTransferRejectsSameUser(t *testing.T) {
	svc := NewWalletService(&mockWalletRepo{}, &mockTxRunner{})

	_, err := svc.Transfer(context.Background(), &TransferReq{FromUserID: 1, ToUserID: 1, Amount: 100})
	if err != errcode.InvalidParams {
		t.Fatalf("expected errcode.InvalidParams, got %v", err)
	}
}
