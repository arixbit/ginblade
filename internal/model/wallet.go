package model

import "time"

// Wallet is a simple account with a balance, used to demonstrate
// transactional writes (InTx) and cached reads (pkg/cache).
type Wallet struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"column:name;type:varchar(255);not null" json:"name"`
	Balance   int64     `gorm:"column:balance;type:bigint;not null;default:0" json:"balance"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"updated_at"`
}

// TableName returns the database table name for Wallet.
func (Wallet) TableName() string {
	return "wallets"
}

// TransferRecord is an audit row written inside the same transaction as the
// balance updates.
type TransferRecord struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	FromID    uint64    `gorm:"column:from_id;not null;index" json:"from_id"`
	ToID      uint64    `gorm:"column:to_id;not null;index" json:"to_id"`
	Amount    int64     `gorm:"column:amount;not null" json:"amount"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"created_at"`
}

// TableName returns the database table name for TransferRecord.
func (TransferRecord) TableName() string {
	return "transfer_records"
}
