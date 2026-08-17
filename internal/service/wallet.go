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

// WalletRepository is the persistence boundary used by WalletService.
type WalletRepository interface {
	Create(ctx context.Context, wallet *model.Wallet) error
	GetByID(ctx context.Context, id uint64) (*model.Wallet, error)
	List(ctx context.Context, limit, offset int) ([]model.Wallet, error)
	Transfer(ctx context.Context, fromID, toID uint64, amount int64) error
}

// WalletCache is the cache boundary used by WalletService for cached reads.
type WalletCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// WalletService handles wallet operations. It demonstrates two patterns the
// skeleton ships but the Example flow does not use: transactional writes
// (delegated to the repository's Transfer) and cache-aside reads (List).
type WalletService struct {
	repo     WalletRepository
	cache    WalletCache
	cacheTTL time.Duration
}

// NewWalletService creates a WalletService. cache is optional; when nil, List
// degrades to a plain database read.
func NewWalletService(repo WalletRepository, cache WalletCache) *WalletService {
	return &WalletService{
		repo:     repo,
		cache:    cache,
		cacheTTL: 60 * time.Second,
	}
}

// CreateWalletReq is the request body for creating a wallet.
type CreateWalletReq struct {
	Name    string `json:"name" binding:"required"`
	Balance int64  `json:"balance"`
}

// Create creates a new wallet.
func (s *WalletService) Create(ctx context.Context, req *CreateWalletReq) (*model.Wallet, error) {
	wallet := model.Wallet{Name: req.Name, Balance: req.Balance}
	if err := s.repo.Create(ctx, &wallet); err != nil {
		applog.FromContext(ctx).Error("failed to create wallet", applog.Error(err))
		return nil, errcode.DatabaseError
	}
	s.invalidateListCache(ctx)
	return &wallet, nil
}

// TransferReq is the request body for a transfer.
type TransferReq struct {
	FromID uint64 `json:"from_id" binding:"required"`
	ToID   uint64 `json:"to_id" binding:"required"`
	Amount int64  `json:"amount" binding:"required,gt=0"`
}

// Transfer moves money between wallets.
func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) error {
	if err := s.repo.Transfer(ctx, req.FromID, req.ToID, req.Amount); err != nil {
		switch {
		case errors.Is(err, repository.ErrInsufficientBalance):
			return errcode.InsufficientBalance
		case errors.Is(err, repository.ErrWalletNotFound):
			return errcode.NotFound
		default:
			applog.FromContext(ctx).Error("failed to transfer", applog.Error(err))
			return errcode.DatabaseError
		}
	}
	s.invalidateListCache(ctx)
	return nil
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

// listVersionKey is bumped on every write so cached lists are invalidated by
// switching to a new cache key (versioned cache-aside).
const listVersionKey = "wallet:list:version"

// List returns wallets, reading through a cache-aside layer when a cache is
// configured: hit → return cached; miss → load from DB, fill cache.
func (s *WalletService) List(ctx context.Context, req *ListWalletsReq) (*ListWalletsRes, error) {
	if req.Limit == 0 {
		req.Limit = 20
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
			_ = s.cache.Set(ctx, key, string(raw), s.cacheTTL)
		}
	}

	return &ListWalletsRes{Wallets: wallets}, nil
}

// listKey returns a cache key scoped to the current list version, so a write
// (which bumps the version) automatically invalidates all cached lists.
func (s *WalletService) listKey(ctx context.Context, limit, offset int) string {
	version := "0"
	if s.cache != nil {
		if cur, err := s.cache.Get(ctx, listVersionKey); err == nil && cur != "" {
			version = cur
		}
	}
	return fmt.Sprintf("wallet:list:v%s:%d:%d", version, limit, offset)
}

// invalidateListCache bumps the list version so subsequent reads miss the
// cache and refetch from the database.
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
