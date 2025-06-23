package controllers

import (
	"fmt"
	"math"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UpdateCart updates the quantity of items in the cart (increment/decrement)
func UpdateCart(c *gin.Context) {
	utils.LogInfo("UpdateCart called")

	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing cart update for user ID: %d", userID)

	var req struct {
		BookID uint   `json:"book_id" binding:"required"`
		Action string `json:"action" binding:"required"` // "increment" or "decrement"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Received update request for book ID: %d, action: %s", req.BookID, req.Action)

	const maxQuantity = 5
	var cart models.Cart
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&cart)
	if cart.ID == 0 {
		utils.LogError("Cart item not found for book ID: %d, user ID: %d", req.BookID, userID)
		utils.NotFound(c, "Cart item not found")
		return
	}
	utils.LogInfo("Found cart item for book ID: %d, current quantity: %d", req.BookID, cart.Quantity)

	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		utils.LogError("Book not found: %d for user ID: %d", req.BookID, userID)
		utils.NotFound(c, "Book not found")
		return
	}

	switch req.Action {
	case "increment":
		if cart.Quantity >= maxQuantity {
			utils.LogError("Max quantity reached for book ID: %d, current: %d, max: %d", req.BookID, cart.Quantity, maxQuantity)
			utils.BadRequest(c, "Max quantity reached", nil)
			return
		}
		if cart.Quantity+1 > book.Stock {
			utils.LogError("Insufficient stock for book ID: %d, requested: %d, available: %d", req.BookID, cart.Quantity+1, book.Stock)
			utils.BadRequest(c, "Book out of stock", nil)
			return
		}
		cart.Quantity++
		db.Save(&cart)
		utils.LogInfo("Incremented quantity for book ID: %d to %d", req.BookID, cart.Quantity)
	case "decrement":
		if cart.Quantity <= 1 {
			db.Delete(&cart)
			utils.LogInfo("Removed cart item for book ID: %d (quantity was 1)", req.BookID)
		} else {
			cart.Quantity--
			db.Save(&cart)
			utils.LogInfo("Decremented quantity for book ID: %d to %d", req.BookID, cart.Quantity)
		}
	default:
		utils.LogError("Invalid action: %s for book ID: %d", req.Action, req.BookID)
		utils.BadRequest(c, "Invalid action", nil)
		return
	}

	// After update, return full cart summary
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	utils.LogInfo("Retrieved %d items from cart for user ID: %d", len(cartItems), userID)

	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Blocked {
			utils.LogInfo("Book ID: %d is inactive or blocked, disabling checkout", cartItems[i].BookID)
			canCheckout = false
		}
		if cartItems[i].Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, cartItems[i].Book.CategoryID)
			if category.Blocked {
				utils.LogInfo("Category ID: %d is blocked, disabling checkout", cartItems[i].Book.CategoryID)
				canCheckout = false
			}
		}
		if cartItems[i].Book.Stock < cartItems[i].Quantity {
			utils.LogInfo("Book ID: %d has insufficient stock, disabling checkout", cartItems[i].BookID)
			canCheckout = false
		}
	}

	var minimalCartItems []gin.H
	subtotal := 0.0
	productDiscountTotal := 0.0
	categoryDiscountTotal := 0.0

	for _, item := range cartItems {
		book := item.Book
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)

		// Calculate product and category discounts separately
		productDiscountAmount := (book.Price * offerBreakdown.ProductOfferPercent / 100) * float64(item.Quantity)
		categoryDiscountAmount := (book.Price * offerBreakdown.CategoryOfferPercent / 100) * float64(item.Quantity)

		// Calculate final price after both discounts
		finalUnitPrice := book.Price - (book.Price * offerBreakdown.ProductOfferPercent / 100) - (book.Price * offerBreakdown.CategoryOfferPercent / 100)
		itemTotal := finalUnitPrice * float64(item.Quantity)

		subtotal += book.Price * float64(item.Quantity) // Original subtotal before discounts
		productDiscountTotal += productDiscountAmount
		categoryDiscountTotal += categoryDiscountAmount

		minimalCartItems = append(minimalCartItems, gin.H{
			"book_id":                book.ID,
			"name":                   book.Name,
			"image_url":              book.ImageURL,
			"quantity":               item.Quantity,
			"original_price":         fmt.Sprintf("%.2f", book.Price),
			"product_offer_percent":  offerBreakdown.ProductOfferPercent,
			"category_offer_percent": offerBreakdown.CategoryOfferPercent,
			"product_discount":       fmt.Sprintf("%.2f", productDiscountAmount),
			"category_discount":      fmt.Sprintf("%.2f", categoryDiscountAmount),
			"final_unit_price":       fmt.Sprintf("%.2f", finalUnitPrice),
			"item_total":             fmt.Sprintf("%.2f", itemTotal),
			"stock_status": func() string {
				if book.Stock < item.Quantity {
					return "Out of Stock"
				}
				if book.Stock <= 3 {
					return "Only a few left"
				}
				return "In Stock"
			}(),
		})
	}

	// Get active coupon if any
	var couponDiscount float64 = 0
	var couponCode string = ""
	var activeUserCoupon models.UserActiveCoupon
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
		}
	}

	// Calculate final total after all discounts
	totalDiscount := productDiscountTotal + categoryDiscountTotal + couponDiscount
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	utils.LogInfo("Cart update completed for user ID: %d, total items: %d, final total: %.2f", userID, len(cartItems), finalTotal)
	utils.Success(c, "Cart updated", gin.H{
		"cart":              minimalCartItems,
		"subtotal":          fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":   fmt.Sprintf("%.2f", math.Round(couponDiscount*100)/100),
		"coupon_code":       couponCode,
		"total_discount":    fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":       fmt.Sprintf("%.2f", finalTotal),
		"can_checkout":      canCheckout,
	})
}
