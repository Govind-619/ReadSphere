package utils

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// OfferBreakdown holds details of both product and category offers
// and the final offer applied
type OfferBreakdown struct {
	ProductOfferPercent  float64 `json:"product_offer_percent"`
	CategoryOfferPercent float64 `json:"category_offer_percent"`
	AppliedOfferPercent  float64 `json:"applied_offer_percent"`
	AppliedOfferType     string  `json:"applied_offer_type"` // "product", "category", or "none"
}

// GetOfferBreakdownForBook returns the product offer, category offer, and the final applied offer for a book
func GetOfferBreakdownForBook(bookID uint, categoryID uint) (OfferBreakdown, error) {
	db := config.DB
	now := time.Now()
	var prodOffer models.ProductOffer
	var catOffer models.CategoryOffer
	prodPercent := 0.0
	catPercent := 0.0

	// Check product-specific offer
	err1 := db.Where("product_id = ? AND active = ? AND start_date <= ? AND end_date >= ?", bookID, true, now, now).First(&prodOffer).Error
	if err1 == nil {
		prodPercent = prodOffer.DiscountPercent
	}
	// Check category-specific offer
	err2 := db.Where("category_id = ? AND active = ? AND start_date <= ? AND end_date >= ?", categoryID, true, now, now).First(&catOffer).Error
	if err2 == nil {
		catPercent = catOffer.DiscountPercent
	}

	// Return both product and category discounts
	return OfferBreakdown{
		ProductOfferPercent:  prodPercent,
		CategoryOfferPercent: catPercent,
		AppliedOfferPercent:  prodPercent + catPercent, // Sum both for total discount
		AppliedOfferType:     "product+category",
	}, nil
}

// Deprecated: Use GetOfferBreakdownForBook instead if you want detailed offer info
func GetBestOfferForBook(bookID uint, categoryID uint) (float64, error) {
	ob, err := GetOfferBreakdownForBook(bookID, categoryID)
	return ob.AppliedOfferPercent, err
}

// ApplyOfferToPrice returns the discounted price given original and discount percent
func ApplyOfferToPrice(original float64, discountPercent float64) float64 {
	if discountPercent <= 0 {
		return original
	}
	discount := (original * discountPercent) / 100.0
	return original - discount
}

// CalculateOfferDetails returns a breakdown of the pricing for a book after applying offers
func CalculateOfferDetails(original float64, bookID uint, categoryID uint) (finalPrice float64, breakdown OfferBreakdown, discountAmount float64, err error) {
	breakdown, err = GetOfferBreakdownForBook(bookID, categoryID)
	if err != nil {
		return original, breakdown, 0, err
	}
	finalPrice = ApplyOfferToPrice(original, breakdown.AppliedOfferPercent)
	discountAmount = original - finalPrice
	return finalPrice, breakdown, discountAmount, nil
}
