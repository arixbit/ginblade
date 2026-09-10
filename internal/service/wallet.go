package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/model"
	"github.com/arixbit/ginblade/internal/repository"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// WalletRepository is the persistence boundary used by WalletService. Every
// method is a single atomic operation; the service composes them and opens
// transactions through TransactionRunner.
type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByID(ctx context.Context, id uint64) (*model.Wallet, error)
	List(ctx context.Context, limit, offset int) ([]model.Wallet, error)
	Debit(ctx context.Context, id uint64, amount int64) error
	Credit(ctx context.Context, id uint64, amount int64) error
	CreateTransferRecord(ctx context.Context, fromID, toID uint64, amount int64) error
}

// WalletCache is the cache boundary used by WalletService for cache-aside
// reads. It is satisfied by pkg/cache.Client and is optional.
type WalletCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

const (
	// walletCacheTTL is how long a cached wallet list stays fresh.
	walletCacheTTL = 60 * time.Second
	// listVersionKey is bumped on every write, which invalidates every cached
	// list at once by moving it to a new cache key (versioned cache-aside).
	listVersionKey = "wallet:list:version"
	// defaultWalletListLimit is used when the request does not set a limit.
	defaultWalletListLimit = 20
)

// WalletService demonstrates two patterns the Example flow does not use:
// transaction orchestration across several repository calls (Transfer) and
// cache-aside reads (List).
type WalletService struct {
	repo  WalletRepository
	cache WalletCache
	tx    TransactionRunner
}

// NewWalletService creates a WalletService. cache is optional: when it is nil,
// reads go straight to the database. tx is required by Transfer.
func NewWalletService(repo WalletRepository, cache WalletCache, tx TransactionRunner) *WalletService {
	return &WalletService{repo: repo, cache: cache, tx: tx}
}

// CreateWalletReq is the request body for creating a wallet.
type CreateWalletReq struct {
	Name    string `json:"name" binding:"required"`
	Balance int64  `json:"balance" binding:"gte=0"`
}

// Create creates a new wallet and invalidates the cached lists.
func (s *WalletService) Create(ctx context.Context, req *CreateWalletReq) (*model.Wallet, error) {
	wallet := model.Wallet{Name: req.Name, Balance: req.Balance}
	if err := s.repo.Create(ctx, &wallet); err != nil {
		applog.FromContext(ctx).Error("failed to create wallet", applog.Error(err))
		return nil, errcode.DatabaseError
	}
	s.invalidateListCache(ctx)
	return &wallet, nil
}

// GetWalletReq identifies a single wallet.
type GetWalletReq struct {
	ID uint64 `uri:"id" binding:"required"`
}

// Get returns one wallet by id.
func (s *WalletService) Get(ctx context.Context, req *GetWalletReq) (*model.Wallet, error) {
	wallet, err := s.repo.GetByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, repository.ErrWalletNotFound) {
			return nil, errcode.NotFound
		}
		applog.FromContext(ctx).Error("failed to get wallet", applog.Error(err))
		return nil, errcode.DatabaseError
	}
	return wallet, nil
}

// TransferReq is the request body for a transfer.
type TransferReq struct {
	FromID uint64 `json:"from_id" binding:"required"`
	ToID   uint64 `json:"to_id" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
}

// TransferRes is the response for a wallet transfer.
type TransferRes struct {
	Success bool `json:"success"`
}

// Transfer moves money between wallets. The debit, the credit, and the audit
// record are orchestrated inside one transaction opened here at the service
// layer — the boundary where a use case coordinates several repository
// operations. If any step fails, the whole transfer rolls back.
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
		switch {
		case errors.Is(err, repository.ErrInsufficientBalance):
			return nil, errcode.InsufficientBalance
		case errors.Is(err, repository.ErrWalletNotFound):
			return nil, errcode.NotFound
		default:
			applog.FromContext(ctx).Error("failed to transfer", applog.Error(err))
			return nil, errcode.DatabaseError
		}
	}

	s.invalidateListCache(ctx)
	return &TransferRes{Success: true}, nil
}

// ListWalletsReq is the request query for listing wallets.
type ListWalletsReq struct {
	Limit  int `form:"limit" binding:"omitempty,min=1,max=100"`
	Offset int `form:"offset" binding:"omitempty,min=0"`
}

// ListWalletsRes is the response for listing wallets.
type ListWalletsRes struct {
	Wallets []model.Wallet `json:"wallets"`
}

// List returns wallets through a cache-aside layer when a cache is configured:
// hit → serve the cached list; miss → load from the database and fill the
// cache. Without a cache it degrades to a plain database read.
func (s *WalletService) List(ctx context.Context, req *ListWalletsReq) (*ListWalletsRes, error) {
	if req.Limit == 0 {
		req.Limit = defaultWalletListLimit
	}

	key := s.listKey(ctx, req.Limit, req.Offset)
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, key); err == nil && cached != "" {
			var wallets []model.Wallet
			if err := json.Unmarshal([]byte(cached), &wallets); err == nil {
				return &ListWalletsRes{Wallets: wallets}, nil
			}
		}
	}

	wallets, err := s.repo.List(ctx, req.Limit, req.Offset)
	if err != nil {
		applog.FromContext(ctx).Error("failed to list wallets", applog.Error(err))
		return nil, errcode.DatabaseError
	}

	if s.cache != nil {
		if raw, err := json.Marshal(wallets); err == nil {
			_ = s.cache.Set(ctx, key, string(raw), walletCacheTTL)
		}
	}

	return &ListWalletsRes{Wallets: wallets}, nil
}

// listKey returns a cache key scoped to the current list version, so a write —
// which bumps the version — automatically invalidates every cached list.
func (s *WalletService) listKey(ctx context.Context, limit, offset int) string {
	version := "0"
	if s.cache != nil {
		if cur, err := s.cache.Get(ctx, listVersionKey); err == nil && cur != "" {
			version = cur
		}
	}
	return fmt.Sprintf("wallet:list:v%s:%d:%d", version, limit, offset)
}

// invalidateListCache bumps the list version so later reads miss the cache and
// refetch from the database.
func (s *WalletService) invalidateListCache(ctx context.Context) {
	if s.cache == nil {
		return
	}
	current := 0
	if cur, err := s.cache.Get(ctx, listVersionKey); err == nil {
		current, _ = strconv.Atoi(cur)
	}
	_ = s.cache.Set(ctx, listVersionKey, strconv.Itoa(current+1), 0)
}
