//go:build integration

package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/repository"
)

// TestWalletTransferIntegration exercises the wallet example against real
// Postgres. The dry-run unit tests can assert the generated SQL, but only a
// real database can prove that a multi-row transfer commits atomically, that a
// failure rolls both legs back, and that the guarded debit refuses to overdraw.
func TestWalletTransferIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	manager := newIsolatedPostgres(t, ctx)
	db := manager.DB()

	if err := db.WithContext(ctx).AutoMigrate(&model.Wallet{}, &model.TransferRecord{}); err != nil {
		t.Fatalf("migrate wallet tables: %v", err)
	}

	repo := repository.NewWalletRepository(db)
	alice := model.Wallet{Name: "alice", Balance: 100}
	bob := model.Wallet{Name: "bob", Balance: 0}
	if err := repo.Create(ctx, &alice); err != nil {
		t.Fatalf("create alice: %v", err)
	}
	if err := repo.Create(ctx, &bob); err != nil {
		t.Fatalf("create bob: %v", err)
	}

	// transfer runs the same three-step orchestration WalletService uses.
	transfer := func(amount int64, failAfter bool) error {
		return repository.InTx(ctx, db, func(txCtx context.Context) error {
			if err := repo.Debit(txCtx, alice.ID, amount); err != nil {
				return err
			}
			if err := repo.Credit(txCtx, bob.ID, amount); err != nil {
				return err
			}
			if err := repo.CreateTransferRecord(txCtx, alice.ID, bob.ID, amount); err != nil {
				return err
			}
			if failAfter {
				return errors.New("force rollback")
			}
			return nil
		})
	}

	t.Run("commit_moves_money_and_writes_audit_row", func(t *testing.T) {
		if err := transfer(30, false); err != nil {
			t.Fatalf("transfer: %v", err)
		}
		assertBalance(t, ctx, db, alice.ID, 70)
		assertBalance(t, ctx, db, bob.ID, 30)
		assertTransferRecordCount(t, ctx, db, 1)
	})

	t.Run("rollback_leaves_both_balances_and_audit_unchanged", func(t *testing.T) {
		if err := transfer(25, true); err == nil {
			t.Fatal("expected the forced rollback to surface an error")
		}
		assertBalance(t, ctx, db, alice.ID, 70)
		assertBalance(t, ctx, db, bob.ID, 30)
		assertTransferRecordCount(t, ctx, db, 1)
	})

	t.Run("debit_refuses_to_overdraw", func(t *testing.T) {
		err := transfer(10_000, false)
		if !errors.Is(err, repository.ErrInsufficientBalance) {
			t.Fatalf("transfer error = %v, want ErrInsufficientBalance", err)
		}
		// The whole transaction rolled back, so no money moved.
		assertBalance(t, ctx, db, alice.ID, 70)
		assertBalance(t, ctx, db, bob.ID, 30)
		assertTransferRecordCount(t, ctx, db, 1)
	})

	t.Run("debit_of_unknown_wallet_reports_insufficient_balance", func(t *testing.T) {
		if err := repo.Debit(ctx, 999_999, 1); !errors.Is(err, repository.ErrInsufficientBalance) {
			t.Fatalf("debit error = %v, want ErrInsufficientBalance", err)
		}
	})

	t.Run("get_missing_wallet_reports_not_found", func(t *testing.T) {
		if _, err := repo.GetByID(ctx, 999_999); !errors.Is(err, repository.ErrWalletNotFound) {
			t.Fatalf("GetByID error = %v, want ErrWalletNotFound", err)
		}
	})
}

func assertBalance(t *testing.T, ctx context.Context, db *gorm.DB, id uint64, want int64) {
	t.Helper()

	var wallet model.Wallet
	if err := db.WithContext(ctx).First(&wallet, id).Error; err != nil {
		t.Fatalf("load wallet %d: %v", id, err)
	}
	if wallet.Balance != want {
		t.Fatalf("wallet %d balance = %d, want %d", id, wallet.Balance, want)
	}
}

func assertTransferRecordCount(t *testing.T, ctx context.Context, db *gorm.DB, want int64) {
	t.Helper()

	var got int64
	if err := db.WithContext(ctx).Model(&model.TransferRecord{}).Count(&got).Error; err != nil {
		t.Fatalf("count transfer records: %v", err)
	}
	if got != want {
		t.Fatalf("transfer record count = %d, want %d", got, want)
	}
}
