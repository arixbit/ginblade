package repository

import (
	"context"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

// WalletRepository persists wallets.
type WalletRepository struct {
	db *gorm.DB
}

// NewWalletRepository creates a WalletRepository.
func NewWalletRepository(db *gorm.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

// GetByUserID returns the wallet of a user.
func (r *WalletRepository) GetByUserID(ctx context.Context, userID uint64) (*model.Wallet, error) {
	var wallet model.Wallet
	if err := dbFromContext(ctx, r.db).WithContext(ctx).
		Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return nil, err
	}
	return &wallet, nil
}

// AddBalance adds delta (positive or negative) to a wallet's balance.
// The caller is responsible for wrapping multiple updates in a transaction.
func (r *WalletRepository) AddBalance(ctx context.Context, userID uint64, delta int64) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("user_id = ?", userID).
		UpdateColumn("balance", gorm.Expr("balance + ?", delta)).Error
}
