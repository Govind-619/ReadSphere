package models

import (
	"time"
)

type WalletTopupOrder struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	UserID          uint      `json:"user_id"`
	RazorpayOrderID string    `json:"razorpay_order_id" gorm:"uniqueIndex"`
	Amount          float64   `json:"amount"`
	Status          string    `json:"status"` // pending, completed, failed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

