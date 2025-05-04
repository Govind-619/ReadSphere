package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CheckoutSummary struct {
	Items      []gin.H `json:"items"`
	Subtotal   float64 `json:"subtotal"`
	Discount   float64 `json:"discount_total"`
	Tax        float64 `json:"tax"`
	FinalTotal float64 `json:"final_total"`
}

func GetCheckoutSummary(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user in context"})
		return
	}
	userID := user.ID
	db := config.DB
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	var items []gin.H
	var subtotal, discountTotal float64
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil {
			continue
		}
		// Sum both product and category offer percent for display and discount
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)
		appliedOfferPercent := offerBreakdown.ProductOfferPercent + offerBreakdown.CategoryOfferPercent
		discountAmount := (book.Price * appliedOfferPercent / 100) * float64(item.Quantity)
		finalUnitPrice := book.Price - (book.Price * appliedOfferPercent / 100)
		itemTotal := finalUnitPrice * float64(item.Quantity)
		items = append(items, gin.H{
			"book_id":    book.ID,
			"name":       book.Name,
			"image_url":  book.ImageURL,
			"quantity":   item.Quantity,
			"original_price": fmt.Sprintf("%.2f", book.Price),
			"product_offer_percent": offerBreakdown.ProductOfferPercent,
			"category_offer_percent": offerBreakdown.CategoryOfferPercent,
			"applied_offer_percent": appliedOfferPercent,
			"applied_offer_type": "product+category",
			"final_unit_price": fmt.Sprintf("%.2f", finalUnitPrice),
			"item_total": fmt.Sprintf("%.2f", itemTotal),
			"discount_amount": fmt.Sprintf("%.2f", discountAmount),
		})
		subtotal += itemTotal
		discountTotal += discountAmount
	}
	tax := 0.05 * subtotal // 5% GST
	finalTotal := subtotal + tax - discountTotal

	// Get wallet balance
	wallet, err := getOrCreateWallet(userID)
	var walletBalance float64 = 0
	if err == nil {
		walletBalance = wallet.Balance
	}

	c.JSON(http.StatusOK, gin.H{
		"items":          items,
		"subtotal":       fmt.Sprintf("%.2f", subtotal),
		"discount":       fmt.Sprintf("%.2f", discountTotal),
		"tax":            fmt.Sprintf("%.2f", tax),
		"final_total":    fmt.Sprintf("%.2f", finalTotal),
		"wallet_balance": fmt.Sprintf("%.2f", walletBalance),
		"can_use_wallet": walletBalance >= finalTotal,
	})
}

func PlaceOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user in context"})
		return
	}
	userID := user.ID
	var req struct {
		AddressID     uint            `json:"address_id"`
		Address       *models.Address `json:"address"`
		PaymentMethod string          `json:"payment_method" binding:"required"`
		CouponCode    string          `json:"coupon_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	db := config.DB
	var address models.Address
	if req.Address != nil {
		// Add new address
		newAddr := *req.Address
		newAddr.UserID = userID
		newAddr.IsDefault = false // Don't override default here
		if err := db.Create(&newAddr).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
			return
		}
		address = newAddr
	} else if req.AddressID != 0 {
		db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address)
		if address.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address not found"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide either address_id or address object"})
		return
	}
	// Fetch cart items
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	if len(cartItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}
	var orderItems []models.OrderItem
	var subtotal, discountTotal float64
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil || book.Stock < item.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available or insufficient stock for book_id " + strconv.FormatUint(uint64(item.BookID), 10)})
			return
		}
		// Sum both product and category offer percent for display and discount
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)
		appliedOfferPercent := offerBreakdown.ProductOfferPercent + offerBreakdown.CategoryOfferPercent
		discountAmount := (book.Price * appliedOfferPercent / 100) * float64(item.Quantity)
		finalUnitPrice := book.Price - (book.Price * appliedOfferPercent / 100)
		itemTotal := finalUnitPrice * float64(item.Quantity)
		orderItems = append(orderItems, models.OrderItem{
			BookID:   book.ID,
			Book:     *book,
			Quantity: item.Quantity,
			Price:    finalUnitPrice,
			Discount: discountAmount,
			Total:    itemTotal,
			// Optionally, you can add offer fields to OrderItem model if you want to persist offer details
		})
		subtotal += itemTotal
		discountTotal += discountAmount
	}

	// Apply coupon discount if provided
	var couponDiscount float64 = 0
	var couponID uint = 0
	var couponCode string = ""

	// Check if a coupon code was provided in the request
	couponCodeToUse := req.CouponCode

	// If no coupon provided, check if user has an active coupon
	if couponCodeToUse == "" {
		var activeUserCoupon models.UserActiveCoupon
		if err := db.Where("user_id = ?", userID).First(&activeUserCoupon).Error; err == nil {
			// Found an active coupon for this user
			couponCodeToUse = activeUserCoupon.Code
		}
	}

	if couponCodeToUse != "" {
		// Find the coupon
		var coupon models.Coupon
		if err := db.Where("code = ?", couponCodeToUse).First(&coupon).Error; err != nil {
			// Coupon not found
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid coupon code"})
			return
		}

		// Check if coupon is active
		if !coupon.Active {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon is not active"})
			return
		}

		// Check if coupon is expired
		if time.Now().After(coupon.Expiry) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon has expired"})
			return
		}

		// Check if coupon usage limit is reached
		if coupon.UsedCount >= coupon.UsageLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon usage limit reached"})
			return
		}

		// Check if user has already used this coupon
		var userCoupon models.UserCoupon
		userUsedCoupon := db.Where("user_id = ? AND coupon_id = ?", userID, coupon.ID).First(&userCoupon).Error == nil
		if userUsedCoupon {
			c.JSON(http.StatusBadRequest, gin.H{"error": "You have already used this coupon"})
			return
		}

		// Check if cart total meets minimum order value
		if subtotal < coupon.MinOrderValue {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Order total does not meet minimum required value for this coupon",
				"min_order_value": coupon.MinOrderValue,
			})
			return
		}

		// Calculate coupon discount
		if coupon.Type == "percent" {
			couponDiscount = (subtotal * coupon.Value) / 100
			if couponDiscount > coupon.MaxDiscount && coupon.MaxDiscount > 0 {
				couponDiscount = coupon.MaxDiscount
			}
		} else {
			couponDiscount = coupon.Value
		}

		couponID = coupon.ID
		couponCode = coupon.Code

		// Update coupon usage count
		db.Model(&coupon).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1))

		// Create user coupon record to mark it as used
		db.Create(&models.UserCoupon{
			UserID:   userID,
			CouponID: coupon.ID,
			UsedAt:   time.Now(),
		})

		// Delete the active coupon since it's now been used
		db.Where("user_id = ?", userID).Delete(&models.UserActiveCoupon{})
	}

	tax := 0.05 * subtotal
	finalTotal := subtotal + tax - couponDiscount

	// Check if using wallet payment method
	if req.PaymentMethod == "wallet" {
		// Get or create wallet for the user
		wallet, err := getOrCreateWallet(userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
			return
		}

		// Check if wallet has enough balance
		if wallet.Balance < finalTotal {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Insufficient wallet balance",
				"wallet_balance":  fmt.Sprintf("%.2f", wallet.Balance),
				"required_amount": fmt.Sprintf("%.2f", finalTotal),
			})
			return
		}

		// Start a database transaction
		tx := db.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
			return
		}

		// Create order
		order := models.Order{
			UserID:         userID,
			AddressID:      address.ID,
			Address:        address,
			TotalAmount:    subtotal,
			Discount:       discountTotal,
			CouponDiscount: couponDiscount,
			CouponID:       couponID,
			CouponCode:     couponCode,
			Tax:            tax,
			FinalTotal:     finalTotal,
			PaymentMethod:  req.PaymentMethod,
			Status:         "Placed",
			OrderItems:     orderItems,
		}

		if err := tx.Create(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
			return
		}

		// Create wallet transaction and ignore return value if not needed
		reference := fmt.Sprintf("ORDER-%d", order.ID)
		description := fmt.Sprintf("Payment for order #%d", order.ID)
		orderID := order.ID

		_, err = createWalletTransaction(wallet.ID, finalTotal, models.TransactionTypeDebit, description, &orderID, reference)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet transaction"})
			return
		}

		// Update wallet balance
		if err := updateWalletBalance(wallet.ID, finalTotal, models.TransactionTypeDebit); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance: " + err.Error()})
			return
		}

		// Reduce stock for each book
		for _, item := range cartItems {
			if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book stock"})
				return
			}
		}

		// Clear cart
		if err := tx.Where("user_id = ?", userID).Delete(&models.Cart{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cart"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
			return
		}

		// Prepare minimal book details
		var booksMinimal []OrderBookMinimal
		for _, item := range order.OrderItems {
			booksMinimal = append(booksMinimal, OrderBookMinimal{
				Name:     item.Book.Name,
				Author:   item.Book.Author,
				Price:    item.Price,
				Quantity: item.Quantity,
				ImageURL: item.Book.ImageURL,
			})
		}

		// Minimal user info
		username := user.Username
		email := user.Email

		// Prepare response
		response := PlaceOrderMinimalResponse{
			Username:    username,
			Email:       email,
			Address:     address,
			Books:       booksMinimal,
			RedirectURL: "/thank-you?order_id=" + strconv.FormatUint(uint64(order.ID), 10),
			ThankYouPage: map[string]interface{}{
				"title":                 "Thank You for Your Order!",
				"subtitle":              "Your order has been placed and is being processed.",
				"order_id":              order.ID,
				"subtotal":              fmt.Sprintf("%.2f", order.TotalAmount),
				"discount":              fmt.Sprintf("%.2f", order.Discount),
				"coupon_discount":       fmt.Sprintf("%.2f", order.CouponDiscount),
				"coupon_code":           couponCode,
				"tax":                   fmt.Sprintf("%.2f", order.Tax),
				"final_total":           fmt.Sprintf("%.2f", order.FinalTotal),
				"payment_method":        order.PaymentMethod,
				"expected_delivery":     "3-7 business days",
				"continue_shopping_url": "/books",
				"wallet_balance":        fmt.Sprintf("%.2f", wallet.Balance),
			},
		}

		c.JSON(http.StatusOK, response)
		return
	}

	order := models.Order{
		UserID:         userID,
		AddressID:      address.ID,
		Address:        address,
		TotalAmount:    subtotal,
		Discount:       discountTotal,
		CouponDiscount: couponDiscount,
		CouponID:       couponID,
		CouponCode:     couponCode,
		Tax:            tax,
		FinalTotal:     finalTotal,
		PaymentMethod:  req.PaymentMethod,
		Status:         "Placed",
		OrderItems:     orderItems,
	}

	db.Create(&order)
	// Reduce stock for each book
	for _, item := range cartItems {
		db.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity))
	}
	// Clear cart only for COD
	if req.PaymentMethod == "cod" || req.PaymentMethod == "COD" {
		db.Where("user_id = ?", userID).Delete(&models.Cart{})
	}
	// Prepare minimal book details
	var booksMinimal []OrderBookMinimal
	for _, item := range order.OrderItems {
		booksMinimal = append(booksMinimal, OrderBookMinimal{
			Name:     item.Book.Name,
			Author:   item.Book.Author,
			Price:    item.Price,
			Quantity: item.Quantity,
			ImageURL: item.Book.ImageURL,
		})
	}
	// Minimal user info
	username := user.Username
	email := user.Email
	// Prepare response
	response := PlaceOrderMinimalResponse{
		Username:    username,
		Email:       email,
		Address:     address,
		Books:       booksMinimal,
		RedirectURL: "/thank-you?order_id=" + strconv.FormatUint(uint64(order.ID), 10),
		ThankYouPage: map[string]interface{}{
			"title":                 "Thank You for Your Order!",
			"subtitle":              "Your order has been placed and is being processed.",
			"order_id":              order.ID,
			"subtotal":              fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":              fmt.Sprintf("%.2f", order.Discount),
			"coupon_discount":       fmt.Sprintf("%.2f", order.CouponDiscount),
			"coupon_code":           couponCode,
			"tax":                   fmt.Sprintf("%.2f", order.Tax),
			"final_total":           fmt.Sprintf("%.2f", order.FinalTotal),
			"payment_method":        order.PaymentMethod,
			"expected_delivery":     "3-7 business days",
			"continue_shopping_url": "/books",
		},
	}
	// If payment method is online, redirect/initiate payment
	if req.PaymentMethod == "online" {
		c.JSON(http.StatusOK, gin.H{
			"redirect_url": "/v1/user/checkout/payment/initiate?order_id=" + strconv.FormatUint(uint64(order.ID), 10),
			"message":      "Please proceed to online payment.",
		})
		return
	}

	// Default: COD or other postpaid methods
	c.JSON(http.StatusOK, response)
}
