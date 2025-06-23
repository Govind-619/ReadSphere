package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// TestPaymentResponse represents the simulated payment response
type TestPaymentResponse struct {
	RazorpayOrderID   string `json:"razorpay_order_id"`
	RazorpayPaymentID string `json:"razorpay_payment_id"`
	RazorpaySignature string `json:"razorpay_signature"`
}

// SimulatePayment simulates a Razorpay payment for testing
func SimulatePayment(c *gin.Context) {
	utils.LogInfo("Starting payment simulation")

	// Get order ID from query parameter
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		utils.LogError("Missing order ID in payment simulation request")
		utils.BadRequest(c, "Order ID is required", nil)
		return
	}
	utils.LogInfo("Processing payment simulation for order ID: %s", orderIDStr)

	// Convert order ID to uint
	var orderID uint
	if _, err := fmt.Sscanf(orderIDStr, "%d", &orderID); err != nil {
		utils.LogError("Invalid order ID format: %s", orderIDStr)
		utils.BadRequest(c, "Invalid order ID format", nil)
		return
	}

	// Get the actual order from database to get the real Razorpay order ID
	db := config.DB
	var order models.Order
	if err := db.Where("id = ?", orderID).First(&order).Error; err != nil {
		utils.LogError("Order not found for ID: %d", orderID)
		utils.NotFound(c, "Order not found")
		return
	}

	// Check if order is still in "Placed" status (not paid yet)
	utils.LogInfo("Checking order status for ID: %d, current status: %s", orderID, order.Status)
	if order.Status != "Placed" {
		utils.LogError("Order ID: %d is not in 'Placed' status. Current status: %s", orderID, order.Status)
		utils.BadRequest(c, "Payment already completed for this order", nil)
		return
	}
	utils.LogInfo("Order ID: %d is in 'Placed' status, proceeding with payment simulation", orderID)

	// Use the actual Razorpay order ID from the database
	razorpayOrderID := order.RazorpayOrderID
	if razorpayOrderID == "" {
		utils.LogError("No Razorpay order ID found for order ID: %d", orderID)
		utils.BadRequest(c, "Payment not initiated for this order", nil)
		return
	}

	utils.LogInfo("Found Razorpay order ID: %s for order ID: %d", razorpayOrderID, orderID)

	// Generate a test payment ID (in real scenario, this comes from Razorpay)
	paymentID := "pay_test_" + fmt.Sprintf("%d", orderID)
	utils.LogDebug("Generated test payment ID: %s", paymentID)

	// Generate signature using Razorpay secret
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := razorpayOrderID + "|" + paymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))
	utils.LogDebug("Generated payment signature: %s", signature)

	// Return simulated payment details with standard response format
	utils.LogInfo("Payment simulation completed successfully for order ID: %d", orderID)

	utils.Success(c, "Payment simulation completed successfully", gin.H{
		"razorpay_order_id":   razorpayOrderID,
		"razorpay_payment_id": paymentID,
		"razorpay_signature":  signature,
	})
}
