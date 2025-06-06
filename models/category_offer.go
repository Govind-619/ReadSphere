package models

import (
	"time"
)

type CategoryOffer struct {
	ID              uint      `gorm:"primaryKey"`
	CategoryID      uint      `gorm:"not null;index"`
	DiscountPercent float64   `gorm:"not null"` // e.g., 10.0 for 10%
	StartDate       time.Time `gorm:"not null"`
	EndDate         time.Time `gorm:"not null"`
	Active          bool      `gorm:"default:true"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
