package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

// newDryRunDBWithUpdates extends the shared dry-run harness with Update
// capture, which the wallet repository needs because it mutates balances.
func newDryRunDBWithUpdates(t *testing.T) (*gorm.DB, *dbCapture) {
	t.Helper()

	capture := &dbCapture{}
	db := newDryRunDB(t, capture)
	if err := db.Callback().Update().After("gorm:update").Register("test:capture_update", func(tx *gorm.DB) {
		if tx.Statement != nil {
			capture.queries = append(capture.queries, tx.Statement.SQL.String())
		}
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	return db, capture
}

func TestWalletRepositoryCreateUsesTransactionFromContext(t *testing.T) {
	baseCapture := &dbCapture{}
	txCapture := &dbCapture{}
	baseDB := newDryRunDB(t, baseCapture)
	txDB := newDryRunDB(t, txCapture)

	repo := NewWalletRepository(baseDB)
	wallet := &model.Wallet{Name: "alice", Balance: 100}

	if err := repo.Create(WithTx(context.Background(), txDB), wallet); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if txCapture.createCalls != 1 {
		t.Fatalf("expected tx db create callback once, got %d", txCapture.createCalls)
	}
	if baseCapture.createCalls != 0 {
		t.Fatalf("expected base db create callback not to run, got %d", baseCapture.createCalls)
	}
}

func TestWalletRepositoryListBuildsOrderedPaginationQuery(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	wallets, err := repo.List(context.Background(), 10, 5)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(wallets) != 0 {
		t.Fatalf("expected dry-run wallets empty, got %d", len(wallets))
	}
	if len(capture.queries) != 1 {
		t.Fatalf("expected 1 query, got %d: %#v", len(capture.queries), capture.queries)
	}

	query := capture.queries[0]
	if !strings.Contains(query, `SELECT * FROM "wallets"`) {
		t.Fatalf("expected list query to select wallets, got %q", query)
	}
	if !strings.Contains(query, "ORDER BY id DESC") {
		t.Fatalf("expected list query to order by id desc, got %q", query)
	}
	if !strings.Contains(query, "LIMIT") || !strings.Contains(query, "OFFSET") {
		t.Fatalf("expected list query to contain limit/offset, got %q", query)
	}
}

// TestWalletRepositoryDebitGuardsBalanceInSQL pins the atomicity guarantee: the
// balance check must live in the UPDATE's WHERE clause, not in a separate read
// followed by a write, so concurrent transfers cannot overdraw a wallet.
func TestWalletRepositoryDebitGuardsBalanceInSQL(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	// A dry run reports zero rows affected, which the repository deliberately
	// maps to ErrInsufficientBalance.
	err := repo.Debit(context.Background(), 7, 25)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("Debit error = %v, want ErrInsufficientBalance", err)
	}

	if len(capture.queries) != 1 {
		t.Fatalf("expected 1 query, got %d: %#v", len(capture.queries), capture.queries)
	}
	query := capture.queries[0]
	if !strings.Contains(query, `UPDATE "wallets"`) {
		t.Fatalf("expected an update on wallets, got %q", query)
	}
	if !strings.Contains(query, "balance >= ") {
		t.Fatalf("debit must guard the balance in SQL, got %q", query)
	}
	if !strings.Contains(query, "balance - ") {
		t.Fatalf("debit must subtract from the balance, got %q", query)
	}
	if !strings.Contains(query, "id = ") {
		t.Fatalf("debit must target a single wallet id, got %q", query)
	}
}

func TestWalletRepositoryCreditAddsToBalance(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	if err := repo.Credit(context.Background(), 7, 25); err != nil {
		t.Fatalf("Credit: %v", err)
	}
	if len(capture.queries) != 1 {
		t.Fatalf("expected 1 query, got %d: %#v", len(capture.queries), capture.queries)
	}

	query := capture.queries[0]
	if !strings.Contains(query, `UPDATE "wallets"`) {
		t.Fatalf("expected an update on wallets, got %q", query)
	}
	if !strings.Contains(query, "balance + ") {
		t.Fatalf("credit must add to the balance, got %q", query)
	}
	if strings.Contains(query, "balance >= ") {
		t.Fatalf("credit must not guard the balance, got %q", query)
	}
}

func TestWalletRepositoryRejectsNonPositiveAmounts(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	for _, amount := range []int64{0, -1} {
		if err := repo.Debit(context.Background(), 1, amount); !errors.Is(err, errNonPositiveAmount) {
			t.Fatalf("Debit(amount=%d) error = %v, want errNonPositiveAmount", amount, err)
		}
		if err := repo.Credit(context.Background(), 1, amount); !errors.Is(err, errNonPositiveAmount) {
			t.Fatalf("Credit(amount=%d) error = %v, want errNonPositiveAmount", amount, err)
		}
	}

	if len(capture.queries) != 0 {
		t.Fatalf("amount validation must happen before touching the database, got %#v", capture.queries)
	}
}

func TestWalletRepositoryCreateTransferRecordWritesAuditRow(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	if err := repo.CreateTransferRecord(context.Background(), 1, 2, 50); err != nil {
		t.Fatalf("CreateTransferRecord: %v", err)
	}
	if capture.createCalls != 1 {
		t.Fatalf("expected create callback once, got %d", capture.createCalls)
	}
	if len(capture.queries) != 1 {
		t.Fatalf("expected 1 query, got %d: %#v", len(capture.queries), capture.queries)
	}
	if query := capture.queries[0]; !strings.Contains(query, `INSERT INTO "transfer_records"`) {
		t.Fatalf("expected an insert into transfer_records, got %q", query)
	}
}

// TestWalletRepositoryGetByIDBuildsPrimaryKeyQuery covers only query
// construction: a dry run cannot return gorm.ErrRecordNotFound, so the
// ErrWalletNotFound translation is asserted against real Postgres in
// tests/integration.
func TestWalletRepositoryGetByIDBuildsPrimaryKeyQuery(t *testing.T) {
	db, capture := newDryRunDBWithUpdates(t)
	repo := NewWalletRepository(db)

	if _, err := repo.GetByID(context.Background(), 404); err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if len(capture.queries) != 1 {
		t.Fatalf("expected 1 query, got %d: %#v", len(capture.queries), capture.queries)
	}
	query := capture.queries[0]
	if !strings.Contains(query, `SELECT * FROM "wallets"`) {
		t.Fatalf("expected a select from wallets, got %q", query)
	}
	if !strings.Contains(query, `"id" = `) || !strings.Contains(query, "LIMIT") {
		t.Fatalf("expected a primary-key lookup, got %q", query)
	}
}
