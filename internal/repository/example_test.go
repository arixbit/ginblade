package repository

import (
	"context"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

type dbCapture struct {
	createCalls int
	queries     []string
}

func newDryRunDB(t *testing.T, capture *dbCapture) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.Open("postgres://u:p@127.0.0.1:5432/db?sslmode=disable"), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm.Open dry run: %v", err)
	}

	if capture == nil {
		return db
	}

	if err := db.Callback().Create().After("gorm:create").Register("test:capture_create", func(tx *gorm.DB) {
		capture.createCalls++
		if tx.Statement != nil {
			capture.queries = append(capture.queries, tx.Statement.SQL.String())
		}
	}); err != nil {
		t.Fatalf("register create callback: %v", err)
	}

	if err := db.Callback().Query().After("gorm:query").Register("test:capture_query", func(tx *gorm.DB) {
		if tx.Statement != nil {
			capture.queries = append(capture.queries, tx.Statement.SQL.String())
		}
	}); err != nil {
		t.Fatalf("register query callback: %v", err)
	}

	return db
}

func TestWithTxAndDBFromContext(t *testing.T) {
	base := new(gorm.DB)
	tx := new(gorm.DB)

	ctx := WithTx(context.Background(), tx)
	if got := dbFromContext(ctx, base); got != tx {
		t.Fatalf("expected tx db from context, got %#v", got)
	}

	if got := dbFromContext(context.Background(), base); got != base {
		t.Fatalf("expected fallback base db, got %#v", got)
	}
}

func TestExampleRepositoryCreateUsesTransactionFromContext(t *testing.T) {
	baseCapture := &dbCapture{}
	txCapture := &dbCapture{}
	baseDB := newDryRunDB(t, baseCapture)
	txDB := newDryRunDB(t, txCapture)

	repo := NewExampleRepository(baseDB)
	example := &model.Example{Name: "example"}

	if err := repo.Create(WithTx(context.Background(), txDB), example); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if txCapture.createCalls != 1 {
		t.Fatalf("expected tx db create callback once, got %d", txCapture.createCalls)
	}
	if baseCapture.createCalls != 0 {
		t.Fatalf("expected base db create callback not to run, got %d", baseCapture.createCalls)
	}
}

func TestExampleRepositoryListBuildsCountAndPaginationQuery(t *testing.T) {
	capture := &dbCapture{}
	db := newDryRunDB(t, capture)
	repo := NewExampleRepository(db)

	examples, total, err := repo.List(context.Background(), 10, 3)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected dry-run total 0, got %d", total)
	}
	if len(examples) != 0 {
		t.Fatalf("expected dry-run examples empty, got %d", len(examples))
	}
	if len(capture.queries) != 2 {
		t.Fatalf("expected 2 queries (count + list), got %d: %#v", len(capture.queries), capture.queries)
	}

	if got := capture.queries[0]; got != `SELECT count(*) FROM "examples"` {
		t.Fatalf("unexpected count query: %q", got)
	}

	listQuery := capture.queries[1]
	if !strings.Contains(listQuery, `SELECT * FROM "examples"`) {
		t.Fatalf("expected list query to select examples, got %q", listQuery)
	}
	if !strings.Contains(listQuery, "ORDER BY id DESC") {
		t.Fatalf("expected list query to order by id desc, got %q", listQuery)
	}
	if !strings.Contains(listQuery, "LIMIT") || !strings.Contains(listQuery, "OFFSET") {
		t.Fatalf("expected list query to contain limit/offset, got %q", listQuery)
	}
}
