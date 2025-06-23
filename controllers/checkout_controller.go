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

	// Debug: Check delivery charges table status
	utils.DebugDeliveryChargesTable()

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
	wallet, err := utils.GetOrCreateWallet(user.ID)
	var walletBalance float64 = 0
	if err == nil {
		walletBalance = wallet.Balance
		utils.LogInfo("Retrieved wallet balance: %.2f for user ID: %d", walletBalance, user.ID)
	} else {
		utils.LogError("Failed to get wallet for user ID: %d: %v", user.ID, err)
	}

	// Get user's default address for delivery charge calculation
	var defaultAddress models.Address
	var deliveryCharge float64 = 0
	var deliveryAvailable bool = true
	var deliveryError string = ""

	if err := config.DB.Where("user_id = ? AND is_default = ?", user.ID, true).First(&defaultAddress).Error; err == nil {
		// Calculate delivery charge based on pincode
		utils.LogInfo("Found default address with pincode: %s for user ID: %d", defaultAddress.PostalCode, user.ID)
		charge, err := utils.GetDeliveryCharge(defaultAddress.PostalCode, cartDetails.FinalTotal)
		if err != nil {
			deliveryError = err.Error()
			deliveryAvailable = false
			utils.LogError("Delivery charge calculation failed for pincode %s, user ID: %d: %v", defaultAddress.PostalCode, user.ID, err)
		} else {
			deliveryCharge = charge
			utils.LogInfo("Calculated delivery charge: %.2f for pincode %s, user ID: %d", deliveryCharge, defaultAddress.PostalCode, user.ID)
		}
	} else {
		// No default address found
		utils.LogInfo("No default address found for user ID: %d, using default delivery charge", user.ID)
		deliveryCharge = 50.0
		deliveryError = "No default address found. Please add a delivery address."
		deliveryAvailable = false
	}

	totalWithDelivery := cartDetails.FinalTotal + deliveryCharge

	utils.LogInfo("Successfully prepared checkout summary for user ID: %d", user.ID)
	utils.Success(c, "Checkout summary retrieved successfully", gin.H{
		"can_checkout":              len(items) > 0,
		"cart":                      items,
		"subtotal":                  fmt.Sprintf("%.2f", cartDetails.Subtotal),
		"product_discount":          fmt.Sprintf("%.2f", cartDetails.ProductDiscount),
		"category_discount":         fmt.Sprintf("%.2f", cartDetails.CategoryDiscount),
		"coupon_code":               cartDetails.CouponCode,
		"coupon_discount":           fmt.Sprintf("%.2f", cartDetails.CouponDiscount),
		"subtotal_without_delivery": fmt.Sprintf("%.2f", cartDetails.FinalTotal),
		"delivery_charge":           fmt.Sprintf("%.2f", deliveryCharge),
		"final_total":               fmt.Sprintf("%.2f", totalWithDelivery),
		"total_discount":            fmt.Sprintf("%.2f", cartDetails.ProductDiscount+cartDetails.CategoryDiscount+cartDetails.CouponDiscount),
		"wallet_balance":            fmt.Sprintf("%.2f", walletBalance),
		"can_use_wallet":            walletBalance >= totalWithDelivery,
		"delivery_available":        deliveryAvailable,
		"delivery_error":            deliveryError,
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

	// Check for existing pending order within 5 minutes
	var existingOrder models.Order
	existingOrderFound := false
	if err := config.DB.Where("user_id = ? AND status = ? AND created_at >= ?", userID, "Placed", time.Now().Add(-5*time.Minute)).Order("created_at desc").First(&existingOrder).Error; err == nil {
		// Check if payment is not completed (for online/razorpay)
		if existingOrder.PaymentMethod == "online" || existingOrder.PaymentMethod == "RAZORPAY" || existingOrder.PaymentMethod == "" {
			// Optionally, check payment status if you have a field for it
			// If payment is not completed, return existing order
			utils.LogInfo("Found existing pending order ID: %d for user ID: %d", existingOrder.ID, userID)
			existingOrderFound = true
		}
	}

	if existingOrderFound {
		// Calculate time left
		waitUntil := existingOrder.CreatedAt.Add(5 * time.Minute)
		timeLeft := int(time.Until(waitUntil).Seconds())
		msg := fmt.Sprintf(
			"You already have a pending order (Order ID: %d). Please complete the payment or wait %d minutes %d seconds before trying a different payment method.",
			existingOrder.ID, timeLeft/60, timeLeft%60,
		)
		utils.BadRequest(c, msg, gin.H{
			"order_id":     existingOrder.ID,
			"redirect_url": fmt.Sprintf("/v1/user/checkout/payment/initiate?order_id=%d", existingOrder.ID),
			"wait_seconds": timeLeft,
		})
		return
	}

	// Only block if the same payment method is pending in the last 15 minutes
	var duplicateOrder models.Order
	if err := config.DB.Where("user_id = ? AND payment_method = ? AND status = ? AND created_at >= ?",
		userID, paymentMethod, "Placed", time.Now().Add(-15*time.Minute)).First(&duplicateOrder).Error; err == nil {
		// Check if there's a pending payment for this order
		var pendingPayment models.Payment
		if err := config.DB.Where("order_id = ? AND status = ?", duplicateOrder.ID, "pending").First(&pendingPayment).Error; err == nil {
			utils.LogError("User already has an order with this payment method pending - User ID: %d", userID)
			utils.BadRequest(c, "You already have an order with this payment method pending. Please complete or cancel that order first.", nil)
			return
		}
	}

	// Get cart details
	cartDetails, err := utils.GetCartDetails(userID)
	if err != nil {
		utils.LogError("Failed to get cart details for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}

	// Calculate delivery charge based on the address being used
	var deliveryCharge float64 = 0
	if req.Address != nil {
		// For new address
		charge, err := utils.GetDeliveryCharge(req.Address.PostalCode, cartDetails.FinalTotal)
		if err != nil {
			utils.LogError("Delivery not available for address - User ID: %d: %v", userID, err)
			utils.BadRequest(c, "Delivery not available for this address", err.Error())
			return
		}
		deliveryCharge = charge
	} else if req.AddressID != 0 {
		// For existing address - we need to get the address first
		// This will be handled later in the function
		deliveryCharge = 50.0 // Default for now
	} else {
		// No address provided, use default
		deliveryCharge = 50.0
	}

	totalWithDelivery := cartDetails.FinalTotal + deliveryCharge
	utils.LogInfo("Calculated delivery charge: %.2f, total with delivery: %.2f for user ID: %d", deliveryCharge, totalWithDelivery, userID)

	// Wallet payment: check balance
	if paymentMethod == "wallet" {
		wallet, err := utils.GetOrCreateWallet(userID)
		if err != nil {
			utils.LogError("Failed to get wallet for user ID: %d: %v", userID, err)
			utils.InternalServerError(c, "Failed to get wallet", err.Error())
			return
		}
		if wallet.Balance < totalWithDelivery {
			utils.LogError("Insufficient wallet balance for user ID: %d. Required: %.2f, Available: %.2f", userID, totalWithDelivery, wallet.Balance)
			utils.BadRequest(c, "Insufficient wallet balance. Please top up your wallet or choose another payment method.", nil)
			return
		}
	}

	// Check COD limit
	if paymentMethod == "cod" {
		if totalWithDelivery > 1000 {
			utils.LogError("COD not available for amount %.2f, user ID: %d", totalWithDelivery, userID)
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
	cartDetails, err = utils.GetCartDetails(userID)
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
		UserID:            userID,
		AddressID:         address.ID,
		Address:           address,
		TotalAmount:       cartDetails.Subtotal,
		Discount:          cartDetails.ProductDiscount + cartDetails.CategoryDiscount,
		CouponDiscount:    cartDetails.CouponDiscount,
		CouponCode:        cartDetails.CouponCode,
		FinalTotal:        cartDetails.FinalTotal,
		DeliveryCharge:    deliveryCharge,
		TotalWithDelivery: totalWithDelivery,
		PaymentMethod: func() string {
			if paymentMethod == "cod" || paymentMethod == "wallet" {
				return paymentMethod
			}
			return "" // For online, leave blank until payment is initiated
		}(),
		Status:     "Placed",
		OrderItems: cartDetails.OrderItems,
	}

	utils.LogInfo("Creating order for user ID: %d, total amount: %.2f, final total: %.2f, delivery charge: %.2f, total with delivery: %.2f",
		userID, order.TotalAmount, order.FinalTotal, order.DeliveryCharge, order.TotalWithDelivery)

	if err := tx.Create(&order).Error; err != nil {
		utils.LogError("Failed to create order for user ID: %d: %v", userID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create order", err.Error())
		return
	}
	utils.LogInfo("Created order ID: %d for user ID: %d", order.ID, userID)

	// Increment coupon used_count if a coupon was used
	if order.CouponCode != "" {
		if err := tx.Model(&models.Coupon{}).Where("code = ?", order.CouponCode).UpdateColumn("used_count", gorm.Expr("used_count + ?", 1)).Error; err != nil {
			utils.LogError("Failed to increment coupon used_count for code %s: %v", order.CouponCode, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update coupon usage count", err.Error())
			return
		}
		utils.LogInfo("Incremented used_count for coupon code: %s", order.CouponCode)
	}

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

	// Process wallet payment deduction
	if paymentMethod == "wallet" {
		// Get wallet within transaction
		var wallet models.Wallet
		if err := tx.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
			utils.LogError("Failed to get wallet for deduction, user ID: %d: %v", userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to get wallet", err.Error())
			return
		}

		// Deduct amount from wallet
		if err := tx.Model(&models.Wallet{}).Where("user_id = ?", userID).
			UpdateColumn("balance", gorm.Expr("balance - ?", totalWithDelivery)).Error; err != nil {
			utils.LogError("Failed to deduct from wallet, user ID: %d: %v", userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to process wallet payment", err.Error())
			return
		}
		utils.LogInfo("Deducted %.2f from wallet for user ID: %d, new balance: %.2f", totalWithDelivery, userID, wallet.Balance-totalWithDelivery)

		// Create wallet transaction record
		walletTransaction := models.WalletTransaction{
			WalletID:    wallet.ID,
			Amount:      -totalWithDelivery, // Negative amount for debit
			Type:        models.TransactionTypeDebit,
			Description: fmt.Sprintf("Payment for order #%d", order.ID),
			OrderID:     &order.ID,
			Reference:   fmt.Sprintf("ORDER-%d", order.ID),
			Status:      models.TransactionStatusCompleted,
		}

		if err := tx.Create(&walletTransaction).Error; err != nil {
			utils.LogError("Failed to create wallet transaction record, user ID: %d: %v", userID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to create wallet transaction", err.Error())
			return
		}
		utils.LogInfo("Created wallet transaction record for order ID: %d", order.ID)
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

	response := gin.H{
		"order_id":        order.ID,
		"payment_method":  order.PaymentMethod,
		"status":          order.Status,
		"subtotal":        fmt.Sprintf("%.2f", cartDetails.FinalTotal),
		"delivery_charge": fmt.Sprintf("%.2f", deliveryCharge),
		"final_total":     fmt.Sprintf("%.2f", totalWithDelivery),
		"delivery_date":   "3-7 working days",
		"shipping_address": gin.H{
			"line1":       order.Address.Line1,
			"line2":       order.Address.Line2,
			"city":        order.Address.City,
			"state":       order.Address.State,
			"country":     order.Address.Country,
			"postal_code": order.Address.PostalCode,
		},
	}

	// Add wallet balance for wallet payments
	if paymentMethod == "wallet" {
		// Get updated wallet balance
		updatedWallet, err := utils.GetOrCreateWallet(userID)
		if err == nil {
			response["wallet_balance"] = fmt.Sprintf("%.2f", updatedWallet.Balance)
			response["amount_deducted"] = fmt.Sprintf("%.2f", totalWithDelivery)
		}
	}

	utils.Success(c, "Thank you for shopping with us! Your order has been placed successfully.", response)
}
