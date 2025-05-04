package models

import (
	"time"

	"gorm.io/gorm"
)

type Coupon struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	Code          string         `gorm:"uniqueIndex:idx_coupons_code_lower" json:"code"`
	Type          string         `json:"type"` // "flat" or "percent"
	Value         float64        `json:"value"`
	MinOrderValue float64        `json:"min_order_value"`
	MaxDiscount   float64        `json:"max_discount"`
	Expiry        time.Time      `json:"expiry"`
	UsageLimit    int            `json:"usage_limit"`
	UsedCount     int            `json:"used_count"`
	Active        bool           `json:"active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

type UserCoupon struct {
	ID       uint      `gorm:"primaryKey" json:"id"`
	UserID   uint      `json:"user_id"`
	CouponID uint      `json:"coupon_id"`
	UsedAt   time.Time `json:"used_at"`
}

// UserActiveCoupon tracks the currently active coupon for each user
type UserActiveCoupon struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id" gorm:"uniqueIndex"` // Only one active coupon per user
	CouponID  uint      `json:"coupon_id"`
	Code      string    `json:"code"`
	AppliedAt time.Time `json:"applied_at"`
}
