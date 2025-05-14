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
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)
	userID := user.ID

	var req struct {
		OrderID uint64 `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request. order_id is required", err.Error())
		return
	}
	db := config.DB
	var order models.Order
	db.Preload("Address").Where("id = ? AND user_id = ?", req.OrderID, userID).First(&order)
	if order.ID == 0 {
		utils.NotFound(c, "Order not found")
		return
	}
	// Razorpay expects amount in paise. Only multiply by 100 if FinalTotal is in rupees (float).
	amountPaise := int(order.FinalTotal * 100)
	if order.FinalTotal > 1000 { // If FinalTotal is already in paise, do not multiply
		amountPaise = int(order.FinalTotal)
	}
	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY"), os.Getenv("RAZORPAY_SECRET"))
	orderData := map[string]interface{}{
		"amount":          amountPaise,
		"currency":        "INR",
		"receipt":         "order_rcptid_" + strconv.FormatUint(uint64(order.ID), 10),
		"payment_capture": 1,
	}
	rzOrder, err := client.Order.Create(orderData, nil)
	if err != nil {
		utils.InternalServerError(c, "Failed to create Razorpay order", err.Error())
		return
	}
	utils.Success(c, "Payment initiated successfully", gin.H{
		"address": gin.H{
			"line1":       order.Address.Line1,
			"line2":       order.Address.Line2,
			"city":        order.Address.City,
			"state":       order.Address.State,
			"country":     order.Address.Country,
			"postal_code": order.Address.PostalCode,
		},
		"amount_display":    fmt.Sprintf("₹%.2f", float64(amountPaise)/100),
		"order_id":          order.ID,
		"razorpay_order_id": rzOrder["id"],
		"key":               os.Getenv("RAZORPAY_KEY"),
		"user": gin.H{
			"name":  user.Username,
			"email": user.Email,
		},
	})
}

// POST /user/checkout/payment/verify
func VerifyRazorpayPayment(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)
	userID := user.ID

	var req struct {
		OrderID           uint   `json:"order_id" binding:"required"`
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
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
		utils.BadRequest(c, "Payment verification failed", gin.H{"retry": true})
		return
	}

	// Find existing order
	db := config.DB
	var order models.Order
	if err := db.Where("id = ? AND user_id = ?", req.OrderID, userID).First(&order).Error; err != nil {
		utils.NotFound(c, "Order not found")
		return
	}

	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Update order status
	order.Status = "Placed"
	order.PaymentMethod = "RAZORPAY"
	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", err.Error())
		return
	}

	// Clear cart
	if err := tx.Where("user_id = ?", userID).Delete(&models.Cart{}).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to clear cart", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	utils.Success(c, "Thank you for your payment! Your order has been placed.", gin.H{
		"order_id":              order.ID,
		"final_total":           fmt.Sprintf("%.2f", order.FinalTotal),
		"payment_method":        order.PaymentMethod,
		"order_details_url":     "/user/orders/" + strconv.FormatUint(uint64(order.ID), 10),
		"continue_shopping_url": "/books",
	})
}

// GetPaymentMethods returns all available payment methods for checkout
func GetPaymentMethods(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user := userVal.(models.User)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get wallet", nil)
		return
	}

	// Fetch final total from query parameters or request to check if wallet has enough balance
	finalTotalStr := c.Query("total")
	var finalTotal float64
	if finalTotalStr != "" {
		finalTotal, _ = strconv.ParseFloat(finalTotalStr, 64)
	}

	// Available payment methods
	paymentMethods := []gin.H{
		{
			"id":          "cod",
			"name":        "Cash on Delivery",
			"description": "Pay when you receive your order",
			"available":   true,
		},
		{
			"id":          "online",
			"name":        "Online Payment",
			"description": "Pay securely with Razorpay",
			"available":   true,
		},
		{
			"id":           "wallet",
			"name":         "Wallet",
			"description":  fmt.Sprintf("Pay using your wallet balance (₹%.2f available)", wallet.Balance),
			"available":    finalTotal == 0 || wallet.Balance >= finalTotal,
			"balance":      fmt.Sprintf("%.2f", wallet.Balance),
			"insufficient": finalTotal > 0 && wallet.Balance < finalTotal,
		},
	}

	utils.Success(c, "Payment methods retrieved successfully", gin.H{
		"payment_methods": paymentMethods,
		"wallet_balance":  fmt.Sprintf("%.2f", wallet.Balance),
	})
}
