package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"os"

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
	// Get order ID from query parameter
	orderID := c.Query("order_id")
	if orderID == "" {
		utils.BadRequest(c, "Order ID is required", nil)
		return
	}

	// Generate a test payment ID (in real scenario, this comes from Razorpay)
	paymentID := "pay_test_" + orderID

	// Generate signature using Razorpay secret
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := orderID + "|" + paymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	signature := hex.EncodeToString(h.Sum(nil))

	// Return simulated payment details with standard response format
	utils.Success(c, "Payment simulation completed successfully", gin.H{
		"razorpay_order_id":   orderID,
		"razorpay_payment_id": paymentID,
		"razorpay_signature":  signature,
	})
}
