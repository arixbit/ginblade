package repository

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/arixbit/ginblade/internal/model"
)

var (
	// ErrInsufficientBalance means a debit would overdraw the source wallet.
	ErrInsufficientBalance = errors.New("repository: insufficient balance")
	// ErrWalletNotFound means the requested wallet does not exist.
	ErrWalletNotFound = errors.New("repository: wallet not found")
	// errNonPositiveAmount rejects a zero or negative amount before it reaches
	// the balance arithmetic.
	errNonPositiveAmount = errors.New("repository: amount must be positive")
)

// WalletRepository persists wallets and transfer records. Every method is a
// single atomic operation; callers compose them inside a transaction via InTx.
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

// GetByID loads a wallet by primary key, returning ErrWalletNotFound when no
// row matches.
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

// Debit subtracts amount from a wallet's balance. The balance condition makes
// the check-and-update atomic, so concurrent transfers cannot overdraw the
// source wallet. It reports ErrInsufficientBalance when the wallet is missing
// or holds less than amount.
func (r *WalletRepository) Debit(ctx context.Context, id uint64, amount int64) error {
	if amount <= 0 {
		return errNonPositiveAmount
	}
	res := dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
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
		return errNonPositiveAmount
	}
	return dbFromContext(ctx, r.db).WithContext(ctx).
		Model(&model.Wallet{}).
		Where("id = ?", id).
		UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error
}

// CreateTransferRecord writes the audit row for a transfer.
func (r *WalletRepository) CreateTransferRecord(ctx context.Context, fromID, toID uint64, amount int64) error {
	return dbFromContext(ctx, r.db).WithContext(ctx).Create(&model.TransferRecord{
		FromID: fromID,
		ToID:   toID,
		Amount: amount,
	}).Error
}
