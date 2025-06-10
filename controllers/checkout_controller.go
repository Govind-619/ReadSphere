package controllers

import (
	"fmt"
	"strings"
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
	FinalTotal float64 `json:"final_total"`
}

func GetCheckoutSummary(c *gin.Context) {
	utils.LogInfo("GetCheckoutSummary called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	utils.LogInfo("Processing checkout summary for user ID: %d", user.ID)

	// Get cart details using the same utility function
	cartDetails, err := utils.GetCartDetails(user.ID)
	if err != nil {
		utils.LogError("Failed to get cart details for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}
	utils.LogInfo("Retrieved cart details for user ID: %d, items count: %d", user.ID, len(cartDetails.OrderItems))

	// Format items for response
	var items []gin.H
	for _, item := range cartDetails.OrderItems {
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(item.BookID, item.Book.CategoryID)

		// Get the item's coupon discount from cart details
		itemCouponDiscount := item.CouponDiscount

		items = append(items, gin.H{
			"book_id":                item.BookID,
			"name":                   item.Book.Name,
			"image_url":              item.Book.ImageURL,
			"quantity":               item.Quantity,
			"original_price":         fmt.Sprintf("%.2f", item.Price),
			"product_offer_percent":  offerBreakdown.ProductOfferPercent,
			"category_offer_percent": offerBreakdown.CategoryOfferPercent,
			"applied_offer_percent":  offerBreakdown.ProductOfferPercent + offerBreakdown.CategoryOfferPercent,
			"applied_offer_type":     "product+category",
			"final_unit_price":       fmt.Sprintf("%.2f", (item.Total-itemCouponDiscount)/float64(item.Quantity)),
			"item_total":             fmt.Sprintf("%.2f", item.Total-itemCouponDiscount),
			"product_discount":       fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount":        fmt.Sprintf("%.2f", itemCouponDiscount),
			"total_discount":         fmt.Sprintf("%.2f", item.Discount+itemCouponDiscount),
		})
	}
	utils.LogInfo("Formatted %d items for checkout summary using cart details", len(items))

	// Get wallet balance
	wallet, err := getOrCreateWallet(user.ID)
	var walletBalance float64 = 0
	if err == nil {
		walletBalance = wallet.Balance
		utils.LogInfo("Retrieved wallet balance: %.2f for user ID: %d", walletBalance, user.ID)
	} else {
		utils.LogError("Failed to get wallet for user ID: %d: %v", user.ID, err)
	}

	utils.LogInfo("Successfully prepared checkout summary for user ID: %d", user.ID)
	utils.Success(c, "Checkout summary retrieved successfully", gin.H{
		"can_checkout":      len(items) > 0,
		"cart":              items,
		"subtotal":          fmt.Sprintf("%.2f", cartDetails.Subtotal),
		"product_discount":  fmt.Sprintf("%.2f", cartDetails.ProductDiscount),
		"category_discount": fmt.Sprintf("%.2f", cartDetails.CategoryDiscount),
		"coupon_code":       cartDetails.CouponCode,
		"coupon_discount":   fmt.Sprintf("%.2f", cartDetails.CouponDiscount),
		"final_total":       fmt.Sprintf("%.2f", cartDetails.FinalTotal),
		"total_discount":    fmt.Sprintf("%.2f", cartDetails.ProductDiscount+cartDetails.CategoryDiscount+cartDetails.CouponDiscount),
		"wallet_balance":    fmt.Sprintf("%.2f", walletBalance),
		"can_use_wallet":    walletBalance >= cartDetails.FinalTotal,
	})
}

func PlaceOrder(c *gin.Context) {
	utils.LogInfo("PlaceOrder called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing order placement for user ID: %d", userID)

	var req struct {
		AddressID     uint            `json:"address_id"`
		Address       *models.Address `json:"address"`
		PaymentMethod string          `json:"payment_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Validate and normalize payment method
	paymentMethod := strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	validMethods := map[string]bool{
		"cod":    true,
		"online": true,
		"wallet": true,
	}
	if !validMethods[paymentMethod] {
		utils.LogError("Invalid payment method '%s' for user ID: %d", paymentMethod, userID)
		utils.BadRequest(c, "Invalid payment method. Must be one of: cod, online, wallet", nil)
		return
	}
	utils.LogInfo("Validated payment method: %s for user ID: %d", paymentMethod, userID)

	// Check if there's an existing order with online payment
	var existingOrder models.Order
	if err := config.DB.Where("user_id = ? AND payment_method = ? AND status = ? AND created_at >= ?",
		userID, "online", "Placed", time.Now().Add(-30*time.Minute)).First(&existingOrder).Error; err == nil {
		// Check if there's a pending payment for this order
		var pendingPayment models.Payment
		if err := config.DB.Where("order_id = ? AND status = ?", existingOrder.ID, "pending").First(&pendingPayment).Error; err == nil {
			utils.LogError("User already has an order with online payment pending - User ID: %d", userID)
			utils.BadRequest(c, "You already have an order with online payment pending. Please complete or cancel that order first.", nil)
			return
		}
	}

	// Check COD limit
	if paymentMethod == "cod" {
		cartDetails, err := utils.GetCartDetails(userID)
		if err != nil {
			utils.LogError("Failed to get cart details for COD check, user ID: %d: %v", userID, err)
			utils.InternalServerError(c, "Failed to get cart details", err.Error())
			return
		}
		if cartDetails.FinalTotal > 1000 {
			utils.LogError("COD not available for amount %.2f, user ID: %d", cartDetails.FinalTotal, userID)
			utils.BadRequest(c, "Cash on Delivery is not available for orders above ₹1000. Please choose online payment or wallet payment.", nil)
			return
		}
		utils.LogInfo("COD amount check passed for user ID: %d", userID)
	}

	db := config.DB
	var address models.Address
	if req.Address != nil {
		// Add new address
		newAddr := *req.Address
		newAddr.UserID = userID
		newAddr.IsDefault = false
		if err := db.Create(&newAddr).Error; err != nil {
			utils.LogError("Failed to create address for user ID: %d: %v", userID, err)
			utils.InternalServerError(c, "Failed to create address", err.Error())
			return
		}
		address = newAddr
		utils.LogInfo("Created new address for user ID: %d", userID)
	} else if req.AddressID != 0 {
		db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address)
		if address.ID == 0 {
			utils.LogError("Address not found, ID: %d, user ID: %d", req.AddressID, userID)
			utils.NotFound(c, "Address not found")
			return
		}
		utils.LogInfo("Retrieved existing address ID: %d for user ID: %d", address.ID, userID)
	} else {
		utils.LogError("No address provided for user ID: %d", userID)
		utils.BadRequest(c, "Provide either address_id or address object", nil)
		return
	}

	// Create order with transaction
	tx := db.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction for user ID: %d: %v", userID, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}
	utils.LogInfo("Started transaction for order placement, user ID: %d", userID)

	// Get cart details
	cartDetails, err := utils.GetCartDetails(userID)
	if err != nil {
		utils.LogError("Failed to get cart details for order placement, user ID: %d: %v", userID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}

	// Check if cart is empty
	if len(cartDetails.OrderItems) == 0 {
		utils.LogError("Empty cart for user ID: %d", userID)
		tx.Rollback()
		utils.BadRequest(c, "Cannot place order with empty cart", nil)
		return
	}
	utils.LogInfo("Retrieved cart details for order placement, items count: %d", len(cartDetails.OrderItems))

	// Validate stock and reduce it for each item
	for _, item := range cartDetails.OrderItems {
		// Lock the book row for update
		var book models.Book
		if err := tx.Set("gorm:pessimistic_lock", true).First(&book, item.BookID).Error; err != nil {
			utils.LogError("Book not found, ID: %d, user ID: %d: %v", item.BookID, userID, err)
			tx.Rollback()
			utils.NotFound(c, fmt.Sprintf("Book with ID %d not found", item.BookID))
			return
		}

		// Check if book has enough stock
		if book.Stock < item.Quantity {
			utils.LogError("Insufficient stock for book '%s', available: %d, requested: %d", book.Name, book.Stock, item.Quantity)
			tx.Rollback()
			utils.BadRequest(c, fmt.Sprintf("Book '%s' does not have enough stock. Available: %d, Requested: %d", book.Name, book.Stock, item.Quantity), nil)
			return
		}

		// Reduce stock
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
			utils.LogError("Failed to update book stock, ID: %d, user ID: %d: %v", item.BookID, userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update book stock", nil)
			return
		}
		utils.LogInfo("Updated stock for book ID: %d, reduced by: %d", item.BookID, item.Quantity)
	}

	order := models.Order{
		UserID:         userID,
		AddressID:      address.ID,
		Address:        address,
		TotalAmount:    cartDetails.Subtotal,
		Discount:       cartDetails.ProductDiscount + cartDetails.CategoryDiscount,
		CouponDiscount: cartDetails.CouponDiscount,
		CouponCode:     cartDetails.CouponCode,
		FinalTotal:     cartDetails.FinalTotal,
		PaymentMethod:  paymentMethod,
		Status:         "Placed",
		OrderItems:     cartDetails.OrderItems,
	}

	utils.LogInfo("Creating order for user ID: %d, total amount: %.2f, final total: %.2f",
		userID, order.TotalAmount, order.FinalTotal)

	if err := tx.Create(&order).Error; err != nil {
		utils.LogError("Failed to create order for user ID: %d: %v", userID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create order", err.Error())
		return
	}
	utils.LogInfo("Created order ID: %d for user ID: %d", order.ID, userID)

	// Clear cart for COD and wallet payments
	if paymentMethod == "cod" || paymentMethod == "wallet" {
		if err := tx.Where("user_id = ?", userID).Delete(&models.Cart{}).Error; err != nil {
			utils.LogError("Failed to clear cart for user ID: %d: %v", userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to clear cart", err.Error())
			return
		}
		utils.LogInfo("Cleared cart for user ID: %d", userID)

		// Clear active coupon
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
			utils.LogError("Failed to clear active coupon for user ID: %d: %v", userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to clear active coupon", err.Error())
			return
		}
		utils.LogInfo("Cleared active coupon for user ID: %d", userID)
	}

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d", order.ID)

	// For online payment, return redirect URL
	if paymentMethod == "online" {
		utils.LogInfo("Returning payment redirect URL for order ID: %d", order.ID)
		utils.Success(c, "Please proceed to payment", gin.H{
			"status": "success",
			"data": gin.H{
				"redirect_url": fmt.Sprintf("/v1/user/checkout/payment/initiate?order_id=%d", order.ID),
				"order_id":     order.ID,
			},
		})
		return
	}

	// For COD and wallet payments, return order confirmation
	utils.LogInfo("Order placed successfully, ID: %d, payment method: %s", order.ID, order.PaymentMethod)
	utils.Success(c, "Thank you for shopping with us! Your order has been placed successfully.", gin.H{
		"order_id":       order.ID,
		"payment_method": order.PaymentMethod,
		"status":         order.Status,
		"final_total":    cartDetails.FinalTotal,
		"delivery_date":  "3-7 working days",
		"shipping_address": gin.H{
			"line1":       order.Address.Line1,
			"line2":       order.Address.Line2,
			"city":        order.Address.City,
			"state":       order.Address.State,
			"country":     order.Address.Country,
			"postal_code": order.Address.PostalCode,
		},
	})
}
