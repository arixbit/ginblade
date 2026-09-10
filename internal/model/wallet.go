package model

import "time"

// Wallet holds the balance of a user. Used by the transfer example to
// demonstrate a cross-row transaction: debiting one wallet and crediting
// another must succeed or fail together.
type Wallet struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"column:user_id;type:bigint;uniqueIndex;not null" json:"user_id"`
	Balance   int64     `gorm:"column:balance;type:bigint;not null;default:0" json:"balance"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamp;default:CURRENT_TIMESTAMP;not null" json:"updated_at"`
}

// TableName returns the database table name for Wallet.
func (Wallet) TableName() string {
	return "wallets"
}
