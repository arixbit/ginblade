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

// Transfer moves amount from one wallet to another atomically. The debit, the
// credit, and the audit record run inside a single InTx transaction, and the
// debit is guarded by a balance condition so concurrent transfers cannot
// overdraw the source wallet.
func (r *WalletRepository) Transfer(ctx context.Context, fromID, toID uint64, amount int64) error {
	if amount <= 0 {
		return errors.New("repository: amount must be positive")
	}
	if fromID == toID {
		return errors.New("repository: cannot transfer to self")
	}

	return InTx(ctx, r.db, func(txCtx context.Context) error {
		db := dbFromContext(txCtx, r.db).WithContext(txCtx)

		// Debit with a balance guard: the UPDATE is atomic, so two concurrent
		// transfers cannot both pass the balance check.
		res := db.Model(&model.Wallet{}).
			Where("id = ? AND balance >= ?", fromID, amount).
			UpdateColumn("balance", gorm.Expr("balance - ?", amount))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInsufficientBalance
		}

		// Credit the destination.
		if err := db.Model(&model.Wallet{}).
			Where("id = ?", toID).
			UpdateColumn("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
			return err
		}

		// Audit record in the same transaction.
		return db.Create(&model.TransferRecord{
			FromID: fromID,
			ToID:   toID,
			Amount: amount,
		}).Error
	})
}
