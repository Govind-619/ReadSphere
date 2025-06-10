package models

import (
	"time"
)

type Payment struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	OrderID         uint      `json:"order_id"`
	RazorpayOrderID string    `json:"razorpay_order_id"`
	Amount          float64   `json:"amount"`
	Status          string    `json:"status"` // pending, completed, failed
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
