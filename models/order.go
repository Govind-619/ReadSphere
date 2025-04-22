package models

import (
	"time"
)

type Order struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `json:"user_id"`
	User         User      `json:"user" gorm:"foreignKey:UserID"`
	AddressID    uint      `json:"address_id"`
	Address      Address   `json:"address" gorm:"foreignKey:AddressID"`
	TotalAmount  float64   `json:"total_amount"`
	Discount     float64   `json:"discount"`
	Tax          float64   `json:"tax"`
	FinalTotal   float64   `json:"final_total"`
	PaymentMethod string   `json:"payment_method"`
	Status       string    `json:"status"`
	ReturnRejectReason string `json:"return_reject_reason,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	OrderItems   []OrderItem `json:"items" gorm:"foreignKey:OrderID"`
}

type OrderItem struct {
	ID        uint    `gorm:"primaryKey" json:"id"`
	OrderID   uint    `json:"order_id"`
	BookID    uint    `json:"book_id"`
	Book      Book    `json:"book"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
	Discount  float64 `json:"discount"`
	Total     float64 `json:"total"`
}
