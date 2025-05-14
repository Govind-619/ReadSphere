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
	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Assume user ID is set in context by auth middleware
	userVal, exists := c.Get("user")
	if !exists {
		tx.Rollback()
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		tx.Rollback()
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID

	var req struct {
		BookID   uint `json:"book_id" binding:"required"`
		Quantity int  `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		tx.Rollback()
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

	// Lock the book row for update to prevent race conditions
	var book models.Book
	if err := tx.Set("gorm:pessimistic_lock", true).First(&book, req.BookID).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Book not found")
		return
	}

	// Check if the item was previously canceled in any order
	var canceledItem models.OrderItem
	if err := tx.Where("book_id = ? AND cancellation_status = ?", req.BookID, "Cancelled").First(&canceledItem).Error; err == nil {
		// Item was previously canceled, check if stock was already restored
		if canceledItem.StockRestored {
			// Stock was already restored, proceed normally
		} else {
			// Update the canceled item to mark stock as restored
			canceledItem.StockRestored = true
			if err := tx.Save(&canceledItem).Error; err != nil {
				tx.Rollback()
				utils.InternalServerError(c, "Failed to update canceled item status", nil)
				return
			}
		}
	}

	// Validate book status
	if !book.IsActive || book.Blocked {
		tx.Rollback()
		utils.BadRequest(c, "Book not available or blocked by admin", nil)
		return
	}

	// Validate category status
	if book.CategoryID != 0 {
		var category models.Category
		if err := tx.First(&category, book.CategoryID).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to check category status", nil)
			return
		}
		if category.Blocked {
			tx.Rollback()
			utils.BadRequest(c, "Category blocked by admin", nil)
			return
		}
	}

	// Check current stock
	if book.Stock < 1 {
		tx.Rollback()
		utils.BadRequest(c, "Book out of stock", nil)
		return
	}

	// Calculate total requested quantity including existing cart items
	var existingCart models.Cart
	var totalRequestedQuantity = req.Quantity

	if err := tx.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&existingCart).Error; err == nil {
		totalRequestedQuantity += existingCart.Quantity
	}

	// Validate total quantity against stock and max limit
	if totalRequestedQuantity > maxQuantity {
		tx.Rollback()
		utils.BadRequest(c, fmt.Sprintf("Cannot add more than %d copies of the same book", maxQuantity), nil)
		return
	}

	if totalRequestedQuantity > book.Stock {
		tx.Rollback()
		utils.BadRequest(c, fmt.Sprintf("Not enough stock. Available: %d", book.Stock), nil)
		return
	}

	// Remove from wishlist if present
	if err := tx.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Wishlist{}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update wishlist", nil)
		return
	}

	// Update or create cart item
	var successMessage string
	if existingCart.ID != 0 {
		existingCart.Quantity = totalRequestedQuantity
		if err := tx.Save(&existingCart).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update cart", nil)
			return
		}
		successMessage = "Cart item quantity updated"
	} else {
		newCart := models.Cart{
			UserID:   userID,
			BookID:   req.BookID,
			Quantity: req.Quantity,
		}
		if err := tx.Create(&newCart).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to add to cart", nil)
			return
		}
		successMessage = "Item added to cart successfully"
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to complete cart update", nil)
		return
	}

	// After successful transaction, get updated cart details
	var cartItems []models.Cart
	if err := config.DB.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
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
			canCheckout = false
		}
		if cartItems[i].Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, cartItems[i].Book.CategoryID)
			if category.Blocked {
				canCheckout = false
			}
		}
		if cartItems[i].Book.Stock < cartItems[i].Quantity {
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

// GetCart retrieves the user's cart and blocks checkout if any item is out of stock
func GetCart(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	var cartItems []models.Cart
	db := config.DB
	// Use table aliases and qualified column names
	db.Table("carts").
		Select("carts.*, books.name as book_name, categories.name as category_name, genres.name as genre_name").
		Joins("LEFT JOIN books ON carts.book_id = books.id").
		Joins("LEFT JOIN categories ON books.category_id = categories.id").
		Joins("LEFT JOIN genres ON books.genre_id = genres.id").
		Where("carts.user_id = ?", userID).
		Find(&cartItems)

	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Blocked {
			canCheckout = false
		}
		if cartItems[i].Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, cartItems[i].Book.CategoryID)
			if category.Blocked {
				canCheckout = false
			}
		}
		if cartItems[i].Book.Stock < cartItems[i].Quantity {
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

	if minimalCartItems == nil {
		minimalCartItems = []gin.H{}
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

	utils.Success(c, "Cart retrieved successfully", gin.H{
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

// UpdateCart updates the quantity of items in the cart (increment/decrement)
func UpdateCart(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	var req struct {
		BookID uint   `json:"book_id" binding:"required"`
		Action string `json:"action" binding:"required"` // "increment" or "decrement"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	const maxQuantity = 5
	var cart models.Cart
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&cart)
	if cart.ID == 0 {
		utils.NotFound(c, "Cart item not found")
		return
	}
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		utils.NotFound(c, "Book not found")
		return
	}
	if req.Action == "increment" {
		if cart.Quantity >= maxQuantity {
			utils.BadRequest(c, "Max quantity reached", nil)
			return
		}
		if cart.Quantity+1 > book.Stock {
			utils.BadRequest(c, "Book out of stock", nil)
			return
		}
		cart.Quantity++
		db.Save(&cart)
	} else if req.Action == "decrement" {
		if cart.Quantity <= 1 {
			db.Delete(&cart)
		} else {
			cart.Quantity--
			db.Save(&cart)
		}
	} else {
		utils.BadRequest(c, "Invalid action", nil)
		return
	}
	// After update, return full cart summary
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Blocked {
			canCheckout = false
		}
		if cartItems[i].Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, cartItems[i].Book.CategoryID)
			if category.Blocked {
				canCheckout = false
			}
		}
		if cartItems[i].Book.Stock < cartItems[i].Quantity {
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

// ClearCart removes all items from the user's cart
func ClearCart(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	db := config.DB
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	utils.Success(c, "Cart cleared successfully", nil)
}

// CheckoutCart attempts to checkout the cart, blocks if any item is out of stock
func CheckoutCart(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	var cartItems []models.Cart
	db := config.DB
	db.Preload("Book").Where("user_id = ?", userID).Find(&cartItems)
	if len(cartItems) == 0 {
		utils.BadRequest(c, "Cart is empty", nil)
		return
	}
	for _, item := range cartItems {
		if !item.Book.IsActive || item.Book.Blocked {
			utils.BadRequest(c, "Book not available or blocked by admin", nil)
			return
		}
		if item.Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, item.Book.CategoryID)
			if category.Blocked {
				utils.BadRequest(c, "Category blocked by admin", nil)
				return
			}
		}
		if item.Book.Stock < item.Quantity {
			utils.BadRequest(c, "Book out of stock", nil)
			return
		}
	}
	// (Order creation logic would go here)
	// For now, just clear cart and return success
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	utils.Success(c, "Checkout successful. Order placed.", nil)
}

// RemoveFromCart removes a product from the cart
func RemoveFromCart(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	var req struct {
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Cart{})
	utils.Success(c, "Product removed from cart successfully", nil)
}
