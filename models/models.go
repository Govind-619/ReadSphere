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
	ProfileImage string    `json:"profile_image"`
	IsBlocked    bool      `json:"is_blocked"`
	IsVerified   bool      `json:"is_verified" gorm:"default:false"`
	IsAdmin      bool      `json:"is_admin" gorm:"default:false"`
	OTP          string    `json:"-"`
	OTPExpiry    time.Time `json:"-"`
	OTPExpiresAt time.Time `json:"-"`
	LastLoginAt  time.Time `json:"last_login_at"`
	GoogleID     string    `gorm:"unique;default:null" json:"google_id"`
	Wallet       Wallet    `json:"wallet,omitempty" gorm:"foreignKey:UserID"`

	Addresses []Address `json:"addresses" gorm:"foreignKey:UserID"`
}

// Admin represents an administrator in the system
type Admin struct {
	gorm.Model
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	Password  string    `json:"-"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	LastLogin time.Time `json:"last_login"`
	IsActive  bool      `json:"is_active" gorm:"default:true"`
}

// Category represents a product category
type Category struct {
	gorm.Model
	Name        string `json:"name"`
	Description string `json:"description"`
	Books       []Book `json:"books,omitempty"`
	Blocked     bool   `json:"blocked" gorm:"default:false"`
}

// Genre represents a book genre
type Genre struct {
	gorm.Model
	Name        string `json:"name" gorm:"uniqueIndex"`
	Description string `json:"description"`
	Books       []Book `json:"books,omitempty"`
}

// Book represents a book in the system
type Book struct {
	gorm.Model
	Name               string      `json:"name"`
	Description        string      `json:"description"`
	Price              float64     `json:"price"`
	OriginalPrice      float64     `json:"original_price"`
	DiscountPercentage int         `json:"discount_percentage"`
	DiscountEndDate    time.Time   `json:"discount_end_date"`
	Stock              int         `json:"stock"`
	CategoryID         uint        `json:"category_id"`
	Category           Category    `json:"category,omitempty" gorm:"foreignKey:CategoryID"`
	GenreID            uint        `json:"genre_id"`
	Genre              Genre       `json:"genre,omitempty" gorm:"foreignKey:GenreID"`
	ImageURL           string      `json:"image_url"`
	BookImages         []BookImage `json:"images" gorm:"foreignKey:BookID"`
	IsActive           bool        `json:"is_active" gorm:"default:true"`
	IsFeatured         bool        `json:"is_featured" gorm:"default:false"`
	Views              int         `json:"views" gorm:"default:0"`
	Reviews            []Review    `json:"reviews,omitempty"`
	AverageRating      float64     `json:"average_rating" gorm:"default:0"`
	TotalReviews       int         `json:"total_reviews" gorm:"default:0"`
	Author             string      `json:"author"`
	Publisher          string      `json:"publisher"`
	ISBN               string      `json:"isbn" gorm:"uniqueIndex"`
	PublicationYear    int         `json:"publication_year"`
	Pages              int         `json:"pages"`
	Language           string      `json:"language" gorm:"default:'English'"`
	Format             string      `json:"format" gorm:"default:'Paperback'"`
	Blocked            bool        `json:"blocked" gorm:"default:false"`
}

// Review represents a book review
type Review struct {
	gorm.Model
	BookID     uint   `json:"book_id"`
	UserID     uint   `json:"user_id"`
	User       User   `json:"user"`
	Rating     int    `json:"rating" gorm:"check:rating >= 1 AND rating <= 5"`
	Comment    string `json:"comment"`
	IsApproved bool   `json:"is_approved" gorm:"default:false"`
}

type Cart struct {
	gorm.Model
	UserID   uint `json:"user_id"`
	User     User `gorm:"foreignKey:UserID" json:"user"`
	BookID   uint `json:"book_id"`
	Book     Book `gorm:"foreignKey:BookID" json:"book"`
	Quantity int  `json:"quantity"`
}

// Order struct moved to order.go. See models/order.go for details.

// UserOTP represents a one-time password for user verification
type UserOTP struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null"`
	Code      string    `json:"code" gorm:"not null"`
	ExpiresAt time.Time `json:"expires_at" gorm:"not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
