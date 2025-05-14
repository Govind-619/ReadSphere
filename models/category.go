package models

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type Category struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	Name         string         `json:"name" gorm:"not null"`
	Description  string         `json:"description"`
	Blocked      bool           `json:"blocked" gorm:"default:false"`
	ReturnWindow int            `json:"return_window" gorm:"default:7"` // Return window in days
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

// BeforeCreate hook to standardize category names
func (c *Category) BeforeCreate(tx *gorm.DB) error {
	c.Name = strings.TrimSpace(c.Name)
	return nil
}

// BeforeUpdate hook to standardize category names
func (c *Category) BeforeUpdate(tx *gorm.DB) error {
	c.Name = strings.TrimSpace(c.Name)
	return nil
}

// BeforeSave hook to ensure name is always in proper format
func (c *Category) BeforeSave(tx *gorm.DB) error {
	c.Name = strings.TrimSpace(c.Name)
	return nil
}
