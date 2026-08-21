package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/arixbit/ginblade/internal/errcode"
	"github.com/arixbit/ginblade/internal/model"
	applog "github.com/arixbit/ginblade/pkg/log"
)

// WalletRepository is the persistence dependency used by WalletService.
type WalletRepository interface {
	GetByUserID(ctx context.Context, userID uint64) (*model.Wallet, error)
	AddBalance(ctx context.Context, userID uint64, delta int64) error
}

// WalletService transfers money between wallets. Both the debit and the
// credit must land atomically, so the transfer runs inside a transaction
// provided by TransactionRunner.
type WalletService struct {
	repo WalletRepository
	tx   TransactionRunner
}

// NewWalletService creates a WalletService.
func NewWalletService(repo WalletRepository, tx TransactionRunner) *WalletService {
	return &WalletService{repo: repo, tx: tx}
}

// TransferReq is the request body for a wallet transfer.
type TransferReq struct {
	FromUserID uint64 `json:"from_user_id" binding:"required"`
	ToUserID   uint64 `json:"to_user_id" binding:"required"`
	Amount     int64  `json:"amount" binding:"required,gt=0"`
}

// TransferRes is the response for a wallet transfer.
type TransferRes struct {
	Success bool `json:"success"`
}

// Transfer moves amount from one user's wallet to another's inside a single
// transaction: if either the debit or the credit fails, both are rolled back.
func (s *WalletService) Transfer(ctx context.Context, req *TransferReq) (*TransferRes, error) {
	if req.FromUserID == req.ToUserID {
		return nil, errcode.InvalidParams
	}

	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.AddBalance(txCtx, req.FromUserID, -req.Amount); err != nil {
			return err
		}
		return s.repo.AddBalance(txCtx, req.ToUserID, req.Amount)
	})
	if err != nil {
		applog.FromContext(ctx).Error("wallet transfer failed", zap.Error(err))
		return nil, errcode.DatabaseError
	}
	return &TransferRes{Success: true}, nil
}
