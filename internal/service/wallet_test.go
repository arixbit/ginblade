package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/repository"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// useNopLogger silences application logging for the duration of a test.
func useNopLogger(t *testing.T) {
	t.Helper()
	t.Cleanup(applog.SetLogger(zap.NewNop()))
}

// assertErrCode fails unless err carries the given business error code.
func assertErrCode(t *testing.T, err error, want errcode.Error) {
	t.Helper()
	var ec errcode.Error
	if !errors.As(err, &ec) || ec.Code() != want.Code() {
		t.Fatalf("expected error code %s, got %v", want.Reason(), err)
	}
}

type mockWalletRepo struct {
	createFunc       func(ctx context.Context, w *model.Wallet) error
	getByIDFunc      func(ctx context.Context, id uint64) (*model.Wallet, error)
	listFunc         func(ctx context.Context, limit, offset int) ([]model.Wallet, error)
	debitFunc        func(ctx context.Context, id uint64, amount int64) error
	creditFunc       func(ctx context.Context, id uint64, amount int64) error
	createRecordFunc func(ctx context.Context, fromID, toID uint64, amount int64) error
}

func (m *mockWalletRepo) Create(ctx context.Context, w *model.Wallet) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, w)
	}
	return nil
}

func (m *mockWalletRepo) GetByID(ctx context.Context, id uint64) (*model.Wallet, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &model.Wallet{ID: id}, nil
}

func (m *mockWalletRepo) List(ctx context.Context, limit, offset int) ([]model.Wallet, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, limit, offset)
	}
	return nil, nil
}

func (m *mockWalletRepo) Debit(ctx context.Context, id uint64, amount int64) error {
	if m.debitFunc != nil {
		return m.debitFunc(ctx, id, amount)
	}
	return nil
}

func (m *mockWalletRepo) Credit(ctx context.Context, id uint64, amount int64) error {
	if m.creditFunc != nil {
		return m.creditFunc(ctx, id, amount)
	}
	return nil
}

func (m *mockWalletRepo) CreateTransferRecord(ctx context.Context, fromID, toID uint64, amount int64) error {
	if m.createRecordFunc != nil {
		return m.createRecordFunc(ctx, fromID, toID, amount)
	}
	return nil
}

// mockTxRunner records whether InTx was used, so tests can assert that the
// service orchestrates its repository calls inside a transaction.
type mockTxRunner struct {
	called bool
	err    error
}

func (m *mockTxRunner) InTx(ctx context.Context, fn func(context.Context) error) error {
	m.called = true
	if m.err != nil {
		return m.err
	}
	return fn(ctx)
}

type mockWalletCache struct {
	store map[string]string
}

func newMockWalletCache() *mockWalletCache {
	return &mockWalletCache{store: make(map[string]string)}
}

func (m *mockWalletCache) Get(_ context.Context, key string) (string, error) {
	return m.store[key], nil
}

func (m *mockWalletCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.store[key] = value
	return nil
}

func TestWalletCreateSuccess(t *testing.T) {
	useNopLogger(t)

	repo := &mockWalletRepo{
		createFunc: func(_ context.Context, w *model.Wallet) error {
			w.ID = 1
			return nil
		},
	}
	cache := newMockWalletCache()
	svc := NewWalletService(repo, cache, &mockTxRunner{})

	wallet, err := svc.Create(context.Background(), &CreateWalletReq{Name: "alice", Balance: 100})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if wallet.ID != 1 || wallet.Balance != 100 {
		t.Fatalf("wallet = %+v", wallet)
	}
	// A write must invalidate cached lists by bumping the version.
	if got := cache.store[listVersionKey]; got != "1" {
		t.Fatalf("list version = %q, want 1", got)
	}
}

func TestWalletCreateDatabaseError(t *testing.T) {
	useNopLogger(t)

	repo := &mockWalletRepo{
		createFunc: func(_ context.Context, _ *model.Wallet) error {
			return errors.New("connection refused")
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.Create(context.Background(), &CreateWalletReq{Name: "x"})
	assertErrCode(t, err, errcode.DatabaseError)
}

func TestWalletGetNotFound(t *testing.T) {
	repo := &mockWalletRepo{
		getByIDFunc: func(_ context.Context, _ uint64) (*model.Wallet, error) {
			return nil, repository.ErrWalletNotFound
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.Get(context.Background(), &GetWalletReq{ID: 42})
	assertErrCode(t, err, errcode.NotFound)
}

func TestWalletGetSuccess(t *testing.T) {
	repo := &mockWalletRepo{
		getByIDFunc: func(_ context.Context, id uint64) (*model.Wallet, error) {
			return &model.Wallet{ID: id, Name: "alice", Balance: 7}, nil
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	wallet, err := svc.Get(context.Background(), &GetWalletReq{ID: 3})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if wallet.ID != 3 || wallet.Name != "alice" || wallet.Balance != 7 {
		t.Fatalf("wallet = %+v", wallet)
	}
}

func TestWalletTransferOrchestratesInsideTx(t *testing.T) {
	var order []string
	repo := &mockWalletRepo{
		debitFunc: func(_ context.Context, _ uint64, _ int64) error {
			order = append(order, "debit")
			return nil
		},
		creditFunc: func(_ context.Context, _ uint64, _ int64) error {
			order = append(order, "credit")
			return nil
		},
	}
	tx := &mockTxRunner{}
	svc := NewWalletService(repo, nil, tx)

	if _, err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 50}); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !tx.called {
		t.Fatal("repository calls must run inside TransactionRunner.InTx")
	}
	want := []string{"debit", "credit"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}
}

func TestWalletTransferRejectsSameWallet(t *testing.T) {
	repo := &mockWalletRepo{}
	tx := &mockTxRunner{}
	svc := NewWalletService(repo, nil, tx)

	_, err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 1, Amount: 10})
	assertErrCode(t, err, errcode.InvalidParams)
	if tx.called {
		t.Fatal("a rejected transfer must not open a transaction")
	}
}

func TestWalletTransferInsufficientBalance(t *testing.T) {
	useNopLogger(t)

	repo := &mockWalletRepo{
		debitFunc: func(_ context.Context, _ uint64, _ int64) error {
			return repository.ErrInsufficientBalance
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 999})
	assertErrCode(t, err, errcode.InsufficientBalance)
}

func TestWalletTransferRepoError(t *testing.T) {
	useNopLogger(t)

	repo := &mockWalletRepo{
		creditFunc: func(_ context.Context, _ uint64, _ int64) error {
			return errors.New("db down")
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 10})
	assertErrCode(t, err, errcode.DatabaseError)
}

func TestWalletListCacheHit(t *testing.T) {
	repo := &mockWalletRepo{}
	cache := newMockWalletCache()
	svc := NewWalletService(repo, cache, &mockTxRunner{})

	wallets := []model.Wallet{{ID: 1, Name: "alice", Balance: 100}}
	raw, err := json.Marshal(wallets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	cache.store[svc.listKey(context.Background(), defaultWalletListLimit, 0)] = string(raw)

	repoCalled := false
	svc.repo = &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			repoCalled = true
			return nil, nil
		},
	}

	res, err := svc.List(context.Background(), &ListWalletsReq{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if repoCalled {
		t.Fatal("repository must not be called on a cache hit")
	}
	if len(res.Wallets) != 1 || res.Wallets[0].Name != "alice" {
		t.Fatalf("unexpected result: %+v", res.Wallets)
	}
}

func TestWalletListCacheMissFillsCache(t *testing.T) {
	repo := &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			return []model.Wallet{{ID: 2, Name: "bob"}}, nil
		},
	}
	cache := newMockWalletCache()
	svc := NewWalletService(repo, cache, &mockTxRunner{})

	res, err := svc.List(context.Background(), &ListWalletsReq{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Wallets) != 1 {
		t.Fatalf("expected 1 wallet, got %d", len(res.Wallets))
	}
	if cache.store[svc.listKey(context.Background(), defaultWalletListLimit, 0)] == "" {
		t.Fatal("cache must be filled after a miss")
	}
}

func TestWalletListWithoutCache(t *testing.T) {
	repo := &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			return []model.Wallet{{ID: 3}}, nil
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	res, err := svc.List(context.Background(), &ListWalletsReq{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(res.Wallets) != 1 {
		t.Fatalf("expected 1 wallet, got %d", len(res.Wallets))
	}
}

func TestWalletListDatabaseError(t *testing.T) {
	useNopLogger(t)

	repo := &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			return nil, errors.New("db down")
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.List(context.Background(), &ListWalletsReq{})
	assertErrCode(t, err, errcode.DatabaseError)
}

func TestWalletListCachesUnderCurrentVersion(t *testing.T) {
	repo := &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			return []model.Wallet{{ID: 4}}, nil
		},
	}
	cache := newMockWalletCache()
	svc := NewWalletService(repo, cache, &mockTxRunner{})

	keyBefore := svc.listKey(context.Background(), defaultWalletListLimit, 0)
	if _, err := svc.List(context.Background(), &ListWalletsReq{}); err != nil {
		t.Fatalf("List: %v", err)
	}

	svc.invalidateListCache(context.Background())

	keyAfter := svc.listKey(context.Background(), defaultWalletListLimit, 0)
	if keyBefore == keyAfter {
		t.Fatalf("cache key must change after invalidation, still %q", keyAfter)
	}
	if cache.store[keyBefore] == "" {
		t.Fatal("expected the pre-invalidation key to still be populated")
	}
}
