package controllers

import (
	"fmt"
	"math"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AddToCart adds a product to the user's cart with validation
func AddToCart(c *gin.Context) {
	utils.LogInfo("AddToCart called")

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Assume user ID is set in context by auth middleware
	userVal, exists := c.Get("user")
	if !exists {
		tx.Rollback()
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		tx.Rollback()
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing add to cart for user ID: %d", userID)

	var req struct {
		BookID   uint `json:"book_id" binding:"required"`
		Quantity int  `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		tx.Rollback()
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	const maxQuantity = 5
	if req.Quantity > maxQuantity {
		req.Quantity = maxQuantity
	}
	utils.LogInfo("Adding book ID: %d with quantity: %d to cart for user ID: %d", req.BookID, req.Quantity, userID)

	// Lock the book row for update to prevent race conditions
	var book models.Book
	if err := tx.Set("gorm:pessimistic_lock", true).First(&book, req.BookID).Error; err != nil {
		tx.Rollback()
		utils.LogError("Book not found: %d for user ID: %d", req.BookID, userID)
		utils.NotFound(c, "Book not found")
		return
	}

	// Check if the item was previously canceled in any order
	var canceledItem models.OrderItem
	if err := tx.Where("book_id = ? AND cancellation_status = ?", req.BookID, "Cancelled").First(&canceledItem).Error; err == nil {
		// Item was previously canceled, check if stock was already restored
		if canceledItem.StockRestored {
			utils.LogInfo("Stock already restored for previously canceled book ID: %d", req.BookID)
		} else {
			// Update the canceled item to mark stock as restored
			canceledItem.StockRestored = true
			if err := tx.Save(&canceledItem).Error; err != nil {
				tx.Rollback()
				utils.LogError("Failed to update canceled item status for book ID: %d: %v", req.BookID, err)
				utils.InternalServerError(c, "Failed to update canceled item status", nil)
				return
			}
			utils.LogInfo("Marked stock as restored for previously canceled book ID: %d", req.BookID)
		}
	}

	// Validate book status
	if !book.IsActive || book.Blocked {
		tx.Rollback()
		utils.LogError("Book ID: %d is not available or blocked", req.BookID)
		utils.BadRequest(c, "Book not available or blocked by admin", nil)
		return
	}

	// Validate category status
	if book.CategoryID != 0 {
		var category models.Category
		if err := tx.First(&category, book.CategoryID).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to check category status for book ID: %d: %v", req.BookID, err)
			utils.InternalServerError(c, "Failed to check category status", nil)
			return
		}
		if category.Blocked {
			tx.Rollback()
			utils.LogError("Category ID: %d is blocked for book ID: %d", book.CategoryID, req.BookID)
			utils.BadRequest(c, "Category blocked by admin", nil)
			return
		}
	}

	// Check current stock
	if book.Stock < 1 {
		tx.Rollback()
		utils.LogError("Book ID: %d is out of stock", req.BookID)
		utils.BadRequest(c, "Book out of stock", nil)
		return
	}

	// Calculate total requested quantity including existing cart items
	var existingCart models.Cart
	var totalRequestedQuantity = req.Quantity

	if err := tx.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&existingCart).Error; err == nil {
		totalRequestedQuantity += existingCart.Quantity
		utils.LogInfo("Found existing cart item for book ID: %d, current quantity: %d", req.BookID, existingCart.Quantity)
	}

	// Validate total quantity against stock and max limit
	if totalRequestedQuantity > maxQuantity {
		tx.Rollback()
		utils.LogError("Quantity exceeds max limit for book ID: %d, requested: %d, max: %d", req.BookID, totalRequestedQuantity, maxQuantity)
		utils.BadRequest(c, fmt.Sprintf("Cannot add more than %d copies of the same book", maxQuantity), nil)
		return
	}

	if totalRequestedQuantity > book.Stock {
		tx.Rollback()
		utils.LogError("Insufficient stock for book ID: %d, requested: %d, available: %d", req.BookID, totalRequestedQuantity, book.Stock)
		utils.BadRequest(c, fmt.Sprintf("Not enough stock. Available: %d", book.Stock), nil)
		return
	}

	// Remove from wishlist if present
	if err := tx.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Wishlist{}).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to remove from wishlist for book ID: %d: %v", req.BookID, err)
		utils.InternalServerError(c, "Failed to update wishlist", nil)
		return
	}

	// Update or create cart item
	var successMessage string
	if existingCart.ID != 0 {
		existingCart.Quantity = totalRequestedQuantity
		if err := tx.Save(&existingCart).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to update cart for book ID: %d: %v", req.BookID, err)
			utils.InternalServerError(c, "Failed to update cart", nil)
			return
		}
		successMessage = "Cart item quantity updated"
		utils.LogInfo("Updated cart quantity for book ID: %d to %d", req.BookID, totalRequestedQuantity)
	} else {
		newCart := models.Cart{
			UserID:   userID,
			BookID:   req.BookID,
			Quantity: req.Quantity,
		}
		if err := tx.Create(&newCart).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to add to cart for book ID: %d: %v", req.BookID, err)
			utils.InternalServerError(c, "Failed to add to cart", nil)
			return
		}
		successMessage = "Item added to cart successfully"
		utils.LogInfo("Added new cart item for book ID: %d with quantity: %d", req.BookID, req.Quantity)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to complete cart update", nil)
		return
	}

	// After successful transaction, get updated cart details
	var cartItems []models.Cart
	if err := config.DB.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		utils.LogError("Failed to fetch updated cart for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to fetch updated cart", nil)
		return
	}

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
	if err := config.DB.Where("user_id = ?", userID).First(&activeUserCoupon).Error; err == nil {
		// Found an active coupon
		var coupon models.Coupon
		if err := config.DB.Where("id = ?", activeUserCoupon.CouponID).First(&coupon).Error; err == nil {
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

	utils.LogInfo("Cart operation completed successfully for user ID: %d, total items: %d, final total: %.2f", userID, len(cartItems), finalTotal)
	utils.Success(c, successMessage, gin.H{
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
