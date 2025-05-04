package models

import (
	"time"

	"gorm.io/gorm"
)

// Wallet represents a user's wallet
type Wallet struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	UserID    uint           `json:"user_id" gorm:"uniqueIndex"`
	Balance   float64        `json:"balance" gorm:"default:0"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// WalletTransaction represents a transaction in a user's wallet
type WalletTransaction struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	WalletID    uint           `json:"wallet_id"`
	Wallet      Wallet         `json:"-" gorm:"foreignKey:WalletID"`
	Amount      float64        `json:"amount"`
	Type        string         `json:"type"` // credit, debit
	Description string         `json:"description"`
	OrderID     *uint          `json:"order_id"`
	Reference   string         `json:"reference"`
	Status      string         `json:"status"` // pending, completed, failed, reversed
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// TransactionType constants
const (
	TransactionTypeCredit = "credit"
	TransactionTypeDebit  = "debit"
)

// TransactionStatus constants
const (
	TransactionStatusPending   = "pending"
	TransactionStatusCompleted = "completed"
	TransactionStatusFailed    = "failed"
	TransactionStatusReversed  = "reversed"
)
