package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
)

// POST /user/checkout/payment/initiate
func InitiateRazorpayPayment(c *gin.Context) {
	utils.LogInfo("InitiateRazorpayPayment called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)
	userID := user.ID
	utils.LogInfo("Processing payment initiation for user ID: %d", userID)

	var req struct {
		OrderID uint64 `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request. order_id is required", err.Error())
		return
	}

	db := config.DB
	var order models.Order
	if err := db.Preload("Address").Where("id = ? AND user_id = ?", req.OrderID, userID).First(&order).Error; err != nil {
		utils.LogError("Order not found for ID: %d, user ID: %d", req.OrderID, userID)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogInfo("Found order ID: %d for user ID: %d", order.ID, userID)

	// Check if payment is already completed
	if order.Status != "Placed" {
		utils.LogError("Order payment already completed - Order ID: %d, Status: %s", order.ID, order.Status)
		utils.BadRequest(c, "Payment for this order has already been completed", nil)
		return
	}

	// Check if there's another pending payment for this order
	if order.PaymentMethod == "RAZORPAY" || order.PaymentMethod == "online" {
		utils.LogError("Payment already initiated for order ID: %d", order.ID)
		utils.BadRequest(c, "A payment is already in progress for this order", nil)
		return
	}

	// Razorpay expects amount in paise. Only multiply by 100 if FinalTotal is in rupees (float).
	amountPaise := int(order.FinalTotal * 100)
	if order.FinalTotal > 1000 { // If FinalTotal is already in paise, do not multiply
		amountPaise = int(order.FinalTotal)
	}
	utils.LogInfo("Processing payment amount: %d paise for order ID: %d", amountPaise, order.ID)

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY"), os.Getenv("RAZORPAY_SECRET"))
	orderData := map[string]interface{}{
		"amount":          amountPaise,
		"currency":        "INR",
		"receipt":         "order_rcptid_" + strconv.FormatUint(uint64(order.ID), 10),
		"payment_capture": 1,
	}
	rzOrder, err := client.Order.Create(orderData, nil)
	if err != nil {
		utils.LogError("Failed to create Razorpay order for order ID: %d: %v", order.ID, err)
		utils.InternalServerError(c, "Failed to create Razorpay order", err.Error())
		return
	}
	utils.LogInfo("Successfully created Razorpay order for order ID: %d", order.ID)

	// Update order with Razorpay order ID
	if err := db.Model(&order).Updates(map[string]interface{}{
		"payment_method":    "RAZORPAY",
		"razorpay_order_id": fmt.Sprintf("%v", rzOrder["id"]),
	}).Error; err != nil {
		utils.LogError("Failed to update order with Razorpay details for order ID: %d: %v", order.ID, err)
		utils.InternalServerError(c, "Failed to update order details", err.Error())
		return
	}

	utils.Success(c, "Payment initiated successfully", gin.H{
		"order": gin.H{
			"id":                order.ID,
			"razorpay_order_id": rzOrder["id"],
			"amount":            fmt.Sprintf("%.2f", order.FinalTotal),
			"delivery_charge":   fmt.Sprintf("%.2f", order.DeliveryCharge),
			"total_amount":      fmt.Sprintf("%.2f", order.TotalWithDelivery),
			"amount_display":    fmt.Sprintf("₹%.2f", order.TotalWithDelivery),
		},
		"address": gin.H{
			"line1":       order.Address.Line1,
			"line2":       order.Address.Line2,
			"city":        order.Address.City,
			"state":       order.Address.State,
			"country":     order.Address.Country,
			"postal_code": order.Address.PostalCode,
		},
		"key": os.Getenv("RAZORPAY_KEY"),
		"user": gin.H{
			"name":  user.Username,
			"email": user.Email,
		},
	})
}

// POST /user/checkout/payment/verify
func VerifyRazorpayPayment(c *gin.Context) {
	utils.LogInfo("VerifyRazorpayPayment called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)
	userID := user.ID
	utils.LogInfo("Processing payment verification for user ID: %d", userID)

	var req struct {
		OrderID           uint   `json:"order_id" binding:"required"`
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Verify signature
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	generatedSignature := hex.EncodeToString(h.Sum(nil))
	if generatedSignature != req.RazorpaySignature {
		utils.LogError("Payment verification failed for order ID: %d, user ID: %d", req.OrderID, userID)
		utils.BadRequest(c, "Payment verification failed", gin.H{"retry": true})
		return
	}
	utils.LogInfo("Payment signature verified for order ID: %d", req.OrderID)

	// Find existing order
	db := config.DB
	var order models.Order
	if err := db.Where("id = ? AND user_id = ?", req.OrderID, userID).First(&order).Error; err != nil {
		utils.LogError("Order not found for ID: %d, user ID: %d: %v", req.OrderID, userID, err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogInfo("Found order ID: %d for user ID: %d", order.ID, userID)

	// Verify that the Razorpay order ID matches
	if order.RazorpayOrderID != req.RazorpayOrderID {
		utils.LogError("Razorpay order ID mismatch for order ID: %d. Expected: %s, Received: %s",
			req.OrderID, order.RazorpayOrderID, req.RazorpayOrderID)
		utils.BadRequest(c, "Invalid Razorpay order ID", nil)
		return
	}
	utils.LogInfo("Razorpay order ID verified for order ID: %d", req.OrderID)

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction for order ID: %d: %v", order.ID, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Update order status
	utils.LogInfo("Updating order ID: %d, current status: %s, new status: Paid", order.ID, order.Status)
	if err := tx.Model(&order).Updates(map[string]interface{}{
		"status":         "Paid",
		"payment_method": "RAZORPAY",
		"payment_status": "completed",
	}).Error; err != nil {
		utils.LogError("Failed to update order ID: %d: %v", order.ID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", err.Error())
		return
	}
	utils.LogInfo("Successfully updated order status to 'Paid' for order ID: %d", order.ID)

	// Clear cart
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

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for order ID: %d: %v", order.ID, err)
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}
	utils.LogInfo("Successfully completed payment verification for order ID: %d", order.ID)

	utils.Success(c, "Thank you for your payment! Your order has been placed.", gin.H{
		"order_id":              order.ID,
		"subtotal":              fmt.Sprintf("%.2f", order.FinalTotal),
		"delivery_charge":       fmt.Sprintf("%.2f", order.DeliveryCharge),
		"final_total":           fmt.Sprintf("%.2f", order.TotalWithDelivery),
		"payment_method":        order.PaymentMethod,
		"order_details_url":     "/user/orders/" + strconv.FormatUint(uint64(order.ID), 10),
		"continue_shopping_url": "/books",
	})
}

// GetPaymentMethods returns all available payment methods for checkout
func GetPaymentMethods(c *gin.Context) {
	utils.LogInfo("GetPaymentMethods called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing payment methods for user ID: %d", user.ID)

	// Get cart details to get the final total
	cartDetails, err := utils.GetCartDetails(user.ID)
	if err != nil {
		utils.LogError("Failed to get cart details for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}
	finalTotal := cartDetails.FinalTotal
	utils.LogInfo("Retrieved final total from cart: %.2f for user ID: %d", finalTotal, user.ID)

	// Get delivery charge for default address
	var deliveryCharge float64 = 50.0 // Default
	var defaultAddress models.Address
	if err := config.DB.Where("user_id = ? AND is_default = ?", user.ID, true).First(&defaultAddress).Error; err == nil {
		charge, err := utils.GetDeliveryCharge(defaultAddress.PostalCode, finalTotal)
		if err == nil {
			deliveryCharge = charge
		}
	}

	totalWithDelivery := finalTotal + deliveryCharge
	utils.LogInfo("Calculated delivery charge: %.2f, total with delivery: %.2f for user ID: %d", deliveryCharge, totalWithDelivery, user.ID)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.LogError("Failed to get wallet for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get wallet", nil)
		return
	}
	utils.LogInfo("Retrieved wallet for user ID: %d, balance: %.2f", user.ID, wallet.Balance)

	// Available payment methods
	paymentMethods := []gin.H{
		{
			"id":          "online",
			"name":        "Online Payment",
			"description": "Pay securely with Razorpay",
			"available":   true,
		},
	}

	// Only add wallet if balance is sufficient or cart is free
	if totalWithDelivery == 0 || wallet.Balance >= totalWithDelivery {
		paymentMethods = append(paymentMethods, gin.H{
			"id":          "wallet",
			"name":        "Wallet",
			"description": fmt.Sprintf("Pay using your wallet balance (₹%.2f available)", wallet.Balance),
			"available":   true,
			"balance":     fmt.Sprintf("%.2f", wallet.Balance),
		})
	}

	// Add COD only if amount is less than or equal to 1000
	if totalWithDelivery <= 1000 {
		utils.LogInfo("Adding COD option for user ID: %d as amount (%.2f) is <= 1000", user.ID, totalWithDelivery)
		paymentMethods = append([]gin.H{
			{
				"id":          "cod",
				"name":        "Cash on Delivery",
				"description": "Pay when you receive your order",
				"available":   true,
			},
		}, paymentMethods...)
	} else {
		utils.LogInfo("COD option not available for user ID: %d as amount (%.2f) is > 1000", user.ID, totalWithDelivery)
	}

	utils.LogInfo("Successfully retrieved payment methods for user ID: %d", user.ID)
	utils.Success(c, "Payment methods retrieved successfully", gin.H{
		"payment_methods":     paymentMethods,
		"wallet_balance":      fmt.Sprintf("%.2f", wallet.Balance),
		"final_total":         fmt.Sprintf("%.2f", finalTotal),
		"delivery_charge":     fmt.Sprintf("%.2f", deliveryCharge),
		"total_with_delivery": fmt.Sprintf("%.2f", totalWithDelivery),
	})
}
