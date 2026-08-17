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

func init() {
	applog.SetLogger(zap.NewNop())
}

type mockWalletRepo struct {
	createFunc       func(ctx context.Context, w *model.Wallet) error
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
	return nil, repository.ErrWalletNotFound
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

// mockTxRunner runs fn directly; it records whether InTx was used so tests can
// assert the service orchestrates repository calls inside a transaction.
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

func (m *mockWalletCache) Get(ctx context.Context, key string) (string, error) {
	v, ok := m.store[key]
	if !ok {
		return "", nil
	}
	return v, nil
}

func (m *mockWalletCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	m.store[key] = value
	return nil
}

func TestWalletCreateSuccess(t *testing.T) {
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
	// 写操作应 bump 列表版本号
	if cache.store[listVersionKey] != "1" {
		t.Fatalf("list version = %q, want 1", cache.store[listVersionKey])
	}
}

func TestWalletCreateDatabaseError(t *testing.T) {
	repo := &mockWalletRepo{
		createFunc: func(_ context.Context, _ *model.Wallet) error {
			return errors.New("connection refused")
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	_, err := svc.Create(context.Background(), &CreateWalletReq{Name: "x"})
	var ec errcode.Error
	if !errors.As(err, &ec) || ec.Code() != errcode.DatabaseError.Code() {
		t.Fatalf("expected DatabaseError, got %v", err)
	}
}

func TestWalletTransferOrchestratesInsideTx(t *testing.T) {
	var order []string
	repo := &mockWalletRepo{
		debitFunc: func(_ context.Context, id uint64, amount int64) error {
			order = append(order, "debit")
			return nil
		},
		creditFunc: func(_ context.Context, id uint64, amount int64) error {
			order = append(order, "credit")
			return nil
		},
		createRecordFunc: func(_ context.Context, _, _ uint64, _ int64) error {
			order = append(order, "record")
			return nil
		},
	}
	tx := &mockTxRunner{}
	svc := NewWalletService(repo, nil, tx)

	if err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 50}); err != nil {
		t.Fatalf("Transfer: %v", err)
	}
	if !tx.called {
		t.Fatal("repository calls must run inside TxRunner.InTx")
	}
	want := []string{"debit", "credit", "record"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}
}

func TestWalletTransferInsufficientBalance(t *testing.T) {
	repo := &mockWalletRepo{
		debitFunc: func(_ context.Context, _ uint64, _ int64) error {
			return repository.ErrInsufficientBalance
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 999})
	var ec errcode.Error
	if !errors.As(err, &ec) || ec.Code() != errcode.InsufficientBalance.Code() {
		t.Fatalf("expected InsufficientBalance, got %v", err)
	}
}

func TestWalletTransferRepoError(t *testing.T) {
	repo := &mockWalletRepo{
		creditFunc: func(_ context.Context, _ uint64, _ int64) error {
			return errors.New("db down")
		},
	}
	svc := NewWalletService(repo, nil, &mockTxRunner{})

	err := svc.Transfer(context.Background(), &TransferReq{FromID: 1, ToID: 2, Amount: 10})
	var ec errcode.Error
	if !errors.As(err, &ec) || ec.Code() != errcode.DatabaseError.Code() {
		t.Fatalf("expected DatabaseError, got %v", err)
	}
}

func TestWalletListCacheHit(t *testing.T) {
	wallets := []model.Wallet{{ID: 1, Name: "alice", Balance: 100}}
	raw, _ := json.Marshal(wallets)

	cache := newMockWalletCache()
	svc := NewWalletService(&mockWalletRepo{}, cache, &mockTxRunner{})
	key := svc.listKey(context.Background(), 20, 0)
	cache.store[key] = string(raw)

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
		t.Fatal("repository should not be called on cache hit")
	}
	if len(res.Wallets) != 1 || res.Wallets[0].Name != "alice" {
		t.Fatalf("unexpected result: %+v", res.Wallets)
	}
}

func TestWalletListCacheMissFillsCache(t *testing.T) {
	wallets := []model.Wallet{{ID: 2, Name: "bob"}}
	repo := &mockWalletRepo{
		listFunc: func(_ context.Context, _, _ int) ([]model.Wallet, error) {
			return wallets, nil
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
	key := svc.listKey(context.Background(), 20, 0)
	if cache.store[key] == "" {
		t.Fatal("cache should be filled after a miss")
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
