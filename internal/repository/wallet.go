package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

var (
	// ErrInsufficientBalance is returned when a transfer would overdraw the
	// source wallet.
	ErrInsufficientBalance = errors.New("repository: insufficient balance")
	// ErrWalletNotFound is returned when a wallet does not exist.
	ErrWalletNotFound = errors.New("repository: wallet not found")
)

// WalletRepository persists wallets and transfer records.
type WalletRepository struct {
	db *gorm.DB
}

// NewWalletRepository creates a WalletRepository.
func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// Create stores a wallet.
func (r *WalletRepository) Create(ctx context.Context, wallet *model.Wallet) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Create(wallet).Error
}

// GetByID loads a wallet by primary key.
func (r *WalletRepository) GetByID(ctx context.Context, id uint64) (*model.Wallet, error) {
	var wallet model.Wallet
	if err := dbFromContext(ctx, r.db).WithContext(ctx).First(&wallet, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWalletNotFound
		}
		return nil, err
	}
	return &wallet, nil
}

// List returns wallets ordered by newest first.
func (r *WalletRepository) List(ctx context.Context, limit, offset int) ([]model.Wallet, error) {
	var wallets []model.Wallet
	if err := dbFromContext(ctx, r.db).WithContext(ctx).
		Order("id DESC").Limit(limit).Offset(offset).Find(&wallets).Error; err != nil {
		return nil, err
	}
	return wallets, nil
}

// Debit subtracts amount from a wallet's balance. It is guarded by a balance
// condition so concurrent transfers cannot overdraw the source wallet. It
// returns ErrInsufficientBalance when the wallet is missing or underfunded.
//
// Debit, Credit, and CreateTransferRecord are individual atomic operations;
// the service layer orchestrates them inside one transaction via
// repository.InTx (see internal/service/wallet.go). Each method uses
// dbFromContext, so it transparently joins a transaction opened by the caller.
func (r *WalletRepository) Debit(ctx context.Context, id uint64, amount int64) error {
	if amount <= 0 {
		return errors.New("repository: amount must be positive")
	}
	db := dbFromContext(ctx, r.db).WithContext(ctx)
	res := db.Model(&model.Wallet{}).
		Where("id = ? AND balance >= ?", id, amount).
		UpdateColumn("balance", gorm.Expr("balance - ?", amount))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrInsufficientBalance
	}
	return nil
}

// Credit adds amount to a wallet's balance.
func (r *WalletRepository) Credit(ctx context.Context, id uint64, amount int64) error {
	if amount <= 0 {
		return errors.New("repository: amount must be positive")
	}
	return dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ?", id).
		UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error
}

// CreateTransferRecord writes an audit row for a transfer.
func (r *WalletRepository) CreateTransferRecord(ctx context.Context, fromID, toID uint64, amount int64) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Create(&model.TransferRecord{
		FromID: fromID,
		ToID:   toID,
		Amount: amount,
	}).Error
}
