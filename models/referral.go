package models

import (
	"time"
	"gorm.io/gorm"
)

type Referral struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	ReferrerUserID  uint           `json:"referrer_user_id"`
	ReferredUserID  uint           `json:"referred_user_id"`
	ReferralCode    string         `json:"referral_code"`
	ReferralToken   string         `json:"referral_token"`
	CouponID        uint           `json:"coupon_id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// For storing referral code at signup
type ReferralSignup struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	UserID        uint           `json:"user_id"`
	ReferralCode  string         `json:"referral_code"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}
