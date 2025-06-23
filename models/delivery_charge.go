package models

import (
	"time"
)

type DeliveryCharge struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Pincode        string    `json:"pincode" gorm:"uniqueIndex;not null"`
	Charge         float64   `json:"charge" gorm:"not null"`
	MinOrderAmount float64   `json:"min_order_amount" gorm:"default:0"`
	IsActive       bool      `json:"is_active" gorm:"default:true"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
