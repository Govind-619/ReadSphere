package models

import (
	"time"

	"gorm.io/gorm"
)

// User represents a regular user in the system
type User struct {
	gorm.Model
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	Email        string    `gorm:"uniqueIndex;not null" json:"email"`
	Password     string    `json:"-"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Phone        string    `json:"phone"`
	IsBlocked    bool      `json:"is_blocked"`
	IsVerified   bool      `json:"is_verified" gorm:"default:false"`
	IsAdmin      bool      `json:"is_admin" gorm:"default:false"`
	OTP          string    `json:"-"`
	OTPExpiry    time.Time `json:"-"`
	OTPExpiresAt time.Time `json:"-"`
	LastLoginAt  time.Time `json:"last_login_at"`
	GoogleID     string    `gorm:"unique;default:null" json:"google_id"`
}

// Admin represents an administrator in the system
type Admin struct {
	gorm.Model
	Email    string `gorm:"uniqueIndex;not null" json:"email"`
	Password string `json:"-"`
}

// Category represents a product category
type Category struct {
	gorm.Model
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Products    []Product `json:"products,omitempty"`
}

// Product represents a product in the system
type Product struct {
	gorm.Model
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	CategoryID  uint     `json:"category_id"`
	Category    Category `json:"category,omitempty"`
	ImageURL    string   `json:"image_url"`
	Images      []string `json:"images" gorm:"type:text[]"`
	IsActive    bool     `json:"is_active" gorm:"default:true"`
	Reviews     []Review `json:"reviews,omitempty"`
}

// Review represents a product review
type Review struct {
	gorm.Model
	UserID    uint    `json:"user_id"`
	User      User    `json:"user,omitempty"`
	ProductID uint    `json:"product_id"`
	Product   Product `json:"product,omitempty"`
	Rating    int     `json:"rating"`
	Comment   string  `json:"comment"`
}

type Cart struct {
	gorm.Model
	UserID    uint    `json:"user_id"`
	User      User    `gorm:"foreignKey:UserID" json:"user"`
	ProductID uint    `json:"product_id"`
	Product   Product `gorm:"foreignKey:ProductID" json:"product"`
	Quantity  int     `json:"quantity"`
}
