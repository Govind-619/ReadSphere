package models

import (
	"time"
)

// PasswordHistory represents the history of user passwords
type PasswordHistory struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UserID    uint      `json:"user_id" gorm:"not null"`
	Password  string    `json:"-" gorm:"not null"` // Password is hashed
	User      User      `json:"-" gorm:"foreignKey:UserID"`
}
