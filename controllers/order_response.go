package controllers

import (
	"github.com/Govind-619/ReadSphere/models"
)

type OrderBookMinimal struct {
	Name     string  `json:"name"`
	Author   string  `json:"author"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	ImageURL string  `json:"image_url"`
}

type OrderBookDetailsMinimal struct {
	ItemID     uint    `json:"item_id"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	CategoryID uint    `json:"category_id"`
	GenreID    uint    `json:"genre_id"`
	Quantity   int     `json:"quantity"`
	Discount   float64 `json:"discount"`
	Total      float64 `json:"total"`
}

type OrderDetailsMinimalResponse struct {
	Email          string                    `json:"email"`
	Name           string                    `json:"name"`
	Address        models.Address            `json:"address"`
	TotalAmount    float64                   `json:"total_amount"`
	Discount       float64                   `json:"discount"`
	CouponDiscount float64                   `json:"coupon_discount"`
	CouponCode     string                    `json:"coupon_code,omitempty"`
	Tax            float64                   `json:"tax"`
	FinalTotal     float64                   `json:"final_total"`
	PaymentMethod  string                    `json:"payment_method"`
	Status         string                    `json:"status"`
	Items          []OrderBookDetailsMinimal `json:"items"`
}

type PlaceOrderMinimalResponse struct {
	Username     string                 `json:"username"`
	Email        string                 `json:"email"`
	Address      models.Address         `json:"address"`
	Books        []OrderBookMinimal     `json:"books"`
	RedirectURL  string                 `json:"redirect_url"`
	ThankYouPage map[string]interface{} `json:"thank_you_page"`
}
