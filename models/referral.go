package models

import (
	"time"

	"gorm.io/gorm"
)

// UserReferralCode represents a permanent referral code for each user
type UserReferralCode struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `json:"user_id" gorm:"uniqueIndex"`
	User         User           `json:"user" gorm:"foreignKey:UserID"`
	ReferralCode string         `json:"referral_code" gorm:"uniqueIndex"`
	IsActive     bool           `json:"is_active" gorm:"default:true"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// ReferralUsage tracks when someone uses a referral code
type ReferralUsage struct {
	ID               uint             `gorm:"primaryKey" json:"id"`
	ReferrerID       uint             `json:"referrer_id"`      // User who owns the referral code
	ReferredUserID   uint             `json:"referred_user_id"` // User who used the referral code
	ReferrerCode     UserReferralCode `json:"referrer_code" gorm:"foreignKey:ReferrerID"`
	ReferredUser     User             `json:"referred_user" gorm:"foreignKey:ReferredUserID"`
	UsedAt           time.Time        `json:"used_at"`
	ReferrerCouponID uint             `json:"referrer_coupon_id"` // Coupon given to referrer
	ReferredCouponID uint             `json:"referred_coupon_id"` // Coupon given to referred user
	ReferrerCoupon   Coupon           `json:"referrer_coupon" gorm:"foreignKey:ReferrerCouponID"`
	ReferredCoupon   Coupon           `json:"referred_coupon" gorm:"foreignKey:ReferredCouponID"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
	DeletedAt        gorm.DeletedAt   `gorm:"index" json:"-"`
}

// Legacy models for backward compatibility (can be removed later)
type Referral struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ReferrerUserID uint           `json:"referrer_user_id"`
	ReferredUserID uint           `json:"referred_user_id"`
	ReferralCode   string         `json:"referral_code"`
	ReferralToken  string         `json:"referral_token"`
	CouponID       uint           `json:"coupon_id"`
	Coupon         Coupon         `json:"coupon" gorm:"foreignKey:CouponID"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// For storing referral code at signup
type ReferralSignup struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	UserID       uint           `json:"user_id"`
	ReferralCode string         `json:"referral_code"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}
