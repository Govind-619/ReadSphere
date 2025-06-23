package models

import (
	"time"

	"gorm.io/gorm"
)

type BlacklistedToken struct {
	gorm.Model
	Token     string    `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time `gorm:"not null"`
}
