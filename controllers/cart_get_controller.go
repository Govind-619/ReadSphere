package controllers

import (
	"fmt"
	"math"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetCart retrieves the user's cart
func GetCart(c *gin.Context) {
	utils.LogInfo("GetCart called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	userID := user.(models.User).ID
	utils.LogInfo("Processing cart retrieval for user ID: %d", userID)

	db := config.DB
	var cartItems []models.Cart
	if err := db.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		utils.LogError("Failed to fetch cart items for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to fetch cart items", nil)
		return
	}
	utils.LogInfo("Found %d items in cart for user ID: %d", len(cartItems), userID)

	var subtotal float64
	var productDiscountTotal float64
	var categoryDiscountTotal float64
	var totalQuantity int
	var minimalCartItems []gin.H
	var canCheckout bool = true

	// First pass: Calculate total quantity and check book statuses
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil {
			utils.LogError("Failed to get book details for ID: %d: %v", item.BookID, err)
			continue
		}

		// Check if book is active
		if !book.IsActive {
			utils.LogInfo("Book ID: %d is inactive", book.ID)
			canCheckout = false
			continue
		}

		// Check if book is blocked
		if book.Blocked {
			utils.LogInfo("Book ID: %d is blocked", book.ID)
			canCheckout = false
			continue
		}

		// Check if book has sufficient stock
		if book.Stock < item.Quantity {
			utils.LogInfo("Insufficient stock for book ID: %d (requested: %d, available: %d)", book.ID, item.Quantity, book.Stock)
			canCheckout = false
			continue
		}

		totalQuantity += item.Quantity
	}

	// Second pass: Calculate discounts and prepare response
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil {
			continue
		}

		// Calculate product and category discounts
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)
		productDiscountAmount := (book.Price * offerBreakdown.ProductOfferPercent / 100) * float64(item.Quantity)
		categoryDiscountAmount := (book.Price * offerBreakdown.CategoryOfferPercent / 100) * float64(item.Quantity)

		subtotal += book.Price * float64(item.Quantity)
		productDiscountTotal += productDiscountAmount
		categoryDiscountTotal += categoryDiscountAmount

		// Get category name
		var category models.Category
		if err := db.First(&category, book.CategoryID).Error; err != nil {
			utils.LogError("Failed to get category for book ID: %d: %v", book.ID, err)
			continue
		}

		minimalCartItems = append(minimalCartItems, gin.H{
			"id":                item.ID,
			"book_id":           book.ID,
			"name":              book.Name,
			"author":            book.Author,
			"price":             book.Price,
			"quantity":          item.Quantity,
			"category":          category.Name,
			"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountAmount*100)/100),
			"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountAmount*100)/100),
		})
	}

	// Check for active coupon
	var activeUserCoupon models.UserActiveCoupon
	var couponCode string
	var couponDiscount float64
	var couponDiscountPerUnit float64

	if err := db.Where("user_id = ?", userID).First(&activeUserCoupon).Error; err == nil {
		// Found an active coupon
		var coupon models.Coupon
		if err := db.Where("id = ?", activeUserCoupon.CouponID).First(&coupon).Error; err == nil {
			couponCode = coupon.Code
			utils.LogInfo("Found active coupon: %s for user ID: %d", couponCode, userID)
			if coupon.Type == "percent" {
				couponDiscount = (subtotal * coupon.Value) / 100
				if couponDiscount > coupon.MaxDiscount {
					couponDiscount = coupon.MaxDiscount
				}
			} else {
				couponDiscount = coupon.Value
			}
			// Calculate coupon discount per unit
			couponDiscountPerUnit = couponDiscount / float64(totalQuantity)
		}
	}

	// Calculate final total after all discounts
	totalDiscount := productDiscountTotal + categoryDiscountTotal + couponDiscount
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	// Update cart items with coupon discount per unit
	for i, item := range minimalCartItems {
		quantity := item["quantity"].(int)
		itemPrice := item["price"].(float64)
		productDiscount := item["product_discount"].(string)
		categoryDiscount := item["category_discount"].(string)

		// Convert string discounts back to float64
		productDiscountFloat, _ := strconv.ParseFloat(productDiscount, 64)
		categoryDiscountFloat, _ := strconv.ParseFloat(categoryDiscount, 64)

		// Calculate total discount per item
		itemCouponDiscount := couponDiscountPerUnit * float64(quantity)
		totalItemDiscount := productDiscountFloat + categoryDiscountFloat + itemCouponDiscount

		// Calculate final item price after all discounts
		finalItemPrice := (itemPrice * float64(quantity)) - totalItemDiscount
		finalItemPricePerUnit := finalItemPrice / float64(quantity)

		minimalCartItems[i]["coupon_discount"] = fmt.Sprintf("%.2f", math.Round(itemCouponDiscount*100)/100)
		minimalCartItems[i]["coupon_discount_per_unit"] = fmt.Sprintf("%.2f", math.Round(couponDiscountPerUnit*100)/100)
		minimalCartItems[i]["final_item_price"] = fmt.Sprintf("%.2f", math.Round(finalItemPricePerUnit*100)/100)
	}

	utils.LogInfo("Cart retrieved successfully for user ID: %d, total items: %d, final total: %.2f", userID, len(cartItems), finalTotal)
	utils.Success(c, "Cart retrieved successfully", gin.H{
		"cart":                     minimalCartItems,
		"subtotal":                 fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":         fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount":        fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":          fmt.Sprintf("%.2f", math.Round(couponDiscount*100)/100),
		"coupon_code":              couponCode,
		"total_discount":           fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":              fmt.Sprintf("%.2f", finalTotal),
		"can_checkout":             canCheckout,
		"coupon_discount_per_unit": fmt.Sprintf("%.2f", math.Round(couponDiscountPerUnit*100)/100),
	})
}
