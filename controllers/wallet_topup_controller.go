package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
)

// InitiateWalletTopup initiates a payment to add money to the wallet
func InitiateWalletTopup(c *gin.Context) {
	utils.LogInfo("InitiateWalletTopup called")
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
	utils.LogInfo("Processing wallet topup request for user ID: %d", userID)

	var req struct {
		Amount float64 `json:"amount" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request body for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request. Amount is required and must be positive", err.Error())
		return
	}
	utils.LogDebug("Received topup request - User ID: %d, Amount: %.2f", userID, req.Amount)

	// Get or create wallet
	wallet, err := utils.GetOrCreateWallet(userID)
	if err != nil {
		utils.LogError("Failed to get wallet for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}
	utils.LogDebug("Retrieved wallet for user ID: %d", userID)

	// Razorpay expects amount in paise
	amountPaise := int(req.Amount * 100)
	utils.LogDebug("Converting amount to paise - Original: %.2f, Paise: %d", req.Amount, amountPaise)

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY"), os.Getenv("RAZORPAY_SECRET"))
	orderData := map[string]interface{}{
		"amount":          amountPaise,
		"currency":        "INR",
		"receipt":         "wallet_topup_" + strconv.FormatUint(uint64(userID), 10) + "_" + time.Now().Format("20060102150405"),
		"payment_capture": 1,
	}
	utils.LogDebug("Creating Razorpay order with data: %+v", orderData)

	rzOrder, err := client.Order.Create(orderData, nil)
	if err != nil {
		utils.LogError("Failed to create Razorpay order for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to create Razorpay order", err.Error())
		return
	}
	utils.LogDebug("Successfully created Razorpay order - Order ID: %v", rzOrder["id"])

	// Save WalletTopupOrder in DB
	walletTopupOrder := models.WalletTopupOrder{
		UserID:          userID,
		RazorpayOrderID: fmt.Sprintf("%v", rzOrder["id"]),
		Amount:          req.Amount,
		Status:          "pending",
	}
	if err := config.DB.Create(&walletTopupOrder).Error; err != nil {
		utils.LogError("Failed to record wallet topup order for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to record wallet topup order", err.Error())
		return
	}
	utils.LogDebug("Created wallet topup order record - Order ID: %s", walletTopupOrder.RazorpayOrderID)

	utils.LogInfo("Successfully initiated wallet topup for user ID: %d", userID)
	utils.Success(c, "Wallet topup order created successfully", gin.H{
		"order": gin.H{
			"id":                walletTopupOrder.ID,
			"razorpay_order_id": rzOrder["id"],
			"amount":            fmt.Sprintf("%.2f", req.Amount),
			"amount_display":    "₹" + fmt.Sprintf("%.2f", float64(amountPaise)/100),
			"payment_type":      "wallet_topup",
		},
		"key": os.Getenv("RAZORPAY_KEY"),
		"user": gin.H{
			"name":  user.Username,
			"email": user.Email,
		},
		"wallet": gin.H{
			"id":      wallet.ID,
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		},
	})
}

// VerifyWalletTopup verifies the payment and adds money to the wallet
func VerifyWalletTopup(c *gin.Context) {
	utils.LogInfo("VerifyWalletTopup called")
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
	utils.LogInfo("Processing wallet topup verification for user ID: %d", userID)

	var req struct {
		OrderID           uint   `json:"order_id" binding:"required"`
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request body for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogDebug("Received verification request - Order ID: %d, Razorpay Order ID: %s, Payment ID: %s", req.OrderID, req.RazorpayOrderID, req.RazorpayPaymentID)

	// Fetch the WalletTopupOrder from DB to get the amount
	var walletTopupOrder models.WalletTopupOrder
	err := config.DB.Where("id = ? AND user_id = ?", req.OrderID, userID).First(&walletTopupOrder).Error
	if err != nil {
		utils.LogError("Failed to fetch wallet topup order - Order ID: %d, User ID: %d: %v", req.OrderID, userID, err)
		utils.BadRequest(c, "Unable to fetch wallet topup order for this order_id", nil)
		return
	}

	// Verify that the Razorpay order ID matches
	if walletTopupOrder.RazorpayOrderID != req.RazorpayOrderID {
		utils.LogError("Razorpay order ID mismatch - Order ID: %d, Expected: %s, Received: %s",
			req.OrderID, walletTopupOrder.RazorpayOrderID, req.RazorpayOrderID)
		utils.BadRequest(c, "Invalid Razorpay order ID", nil)
		return
	}

	// Check if order is still pending
	if walletTopupOrder.Status != "pending" {
		utils.LogError("Wallet topup order is not in pending status - Order ID: %d, Status: %s", req.OrderID, walletTopupOrder.Status)
		utils.BadRequest(c, "Payment already completed for this wallet topup order", nil)
		return
	}

	amount := walletTopupOrder.Amount
	if amount <= 0 {
		utils.LogError("Invalid amount for wallet topup order - Order ID: %d, Amount: %.2f", req.OrderID, amount)
		utils.BadRequest(c, "Invalid amount for this wallet topup order", nil)
		return
	}
	utils.LogDebug("Retrieved wallet topup order - Amount: %.2f", amount)

	// Verify signature
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	generatedSignature := hex.EncodeToString(h.Sum(nil))
	if generatedSignature != req.RazorpaySignature {
		utils.LogError("Payment verification failed - Order ID: %d, Razorpay Order ID: %s, Expected: %s, Got: %s",
			req.OrderID, req.RazorpayOrderID, generatedSignature, req.RazorpaySignature)
		utils.BadRequest(c, "Payment verification failed", gin.H{"retry": true})
		return
	}
	utils.LogDebug("Successfully verified payment signature for order ID: %d", req.OrderID)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", req.OrderID, tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", tx.Error.Error())
		return
	}
	utils.LogDebug("Started transaction for order ID: %d", req.OrderID)

	// Get or create wallet
	wallet, err := utils.GetOrCreateWallet(userID)
	if err != nil {
		tx.Rollback()
		utils.LogError("Failed to get wallet for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}
	utils.LogDebug("Retrieved wallet for user ID: %d", userID)

	// Create a wallet transaction
	reference := fmt.Sprintf("TOPUP-%s", req.RazorpayPaymentID)
	description := "Wallet topup via Razorpay"
	utils.LogDebug("Creating wallet transaction - Reference: %s, Amount: %.2f", reference, amount)

	transaction, err := utils.CreateWalletTransaction(wallet.ID, amount, models.TransactionTypeCredit, description, nil, reference)
	if err != nil {
		tx.Rollback()
		utils.LogError("Failed to create wallet transaction for order ID: %d: %v", req.OrderID, err)
		utils.InternalServerError(c, "Failed to create transaction", err.Error())
		return
	}
	utils.LogDebug("Created wallet transaction ID: %d", transaction.ID)

	// Update wallet balance
	if err := utils.UpdateWalletBalance(wallet.ID, amount, models.TransactionTypeCredit); err != nil {
		tx.Rollback()
		utils.LogError("Failed to update wallet balance for wallet ID: %d: %v", wallet.ID, err)
		utils.InternalServerError(c, "Failed to update wallet balance", err.Error())
		return
	}
	utils.LogDebug("Updated wallet balance for wallet ID: %d", wallet.ID)

	// Update wallet topup order status
	walletTopupOrder.Status = "completed"
	if err := tx.Save(&walletTopupOrder).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update topup order status for order ID: %d: %v", req.OrderID, err)
		utils.InternalServerError(c, "Failed to update topup order status", err.Error())
		return
	}
	utils.LogDebug("Updated wallet topup order status to completed for order ID: %d", req.OrderID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for order ID: %d: %v", req.OrderID, err)
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}
	utils.LogDebug("Successfully committed transaction for order ID: %d", req.OrderID)

	// Get updated wallet
	updatedWallet, err := utils.GetOrCreateWallet(userID)
	if err != nil {
		utils.LogError("Failed to get updated wallet for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to get updated wallet", err.Error())
		return
	}
	utils.LogDebug("Retrieved updated wallet balance: %.2f", updatedWallet.Balance)

	utils.LogInfo("Successfully completed wallet topup for user ID: %d", userID)
	utils.Success(c, "Money added to wallet successfully!", gin.H{
		"order": gin.H{
			"id":                  req.OrderID,
			"razorpay_order_id":   req.RazorpayOrderID,
			"razorpay_payment_id": req.RazorpayPaymentID,
			"amount":              fmt.Sprintf("%.2f", amount),
			"amount_display":      "₹" + fmt.Sprintf("%.2f", amount),
			"status":              "completed",
			"payment_type":        "wallet_topup",
		},
		"wallet": gin.H{
			"id":               wallet.ID,
			"balance":          fmt.Sprintf("%.2f", updatedWallet.Balance),
			"amount_added":     fmt.Sprintf("%.2f", amount),
			"transaction_id":   transaction.ID,
			"transaction_date": transaction.CreatedAt.Format("2006-01-02 15:04:05"),
			"reference":        reference,
		},
		"user": gin.H{
			"id":    user.ID,
			"name":  user.Username,
			"email": user.Email,
		},
	})
}

// SimulateWalletTopupPayment simulates a Razorpay payment for wallet topup testing
func SimulateWalletTopupPayment(c *gin.Context) {
	utils.LogInfo("Starting wallet topup payment simulation")

	// Get user from context
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

	// Get order ID from query parameter
	orderIDStr := c.Query("order_id")
	if orderIDStr == "" {
		utils.LogError("Missing order ID in wallet topup payment simulation request")
		utils.BadRequest(c, "Order ID is required", nil)
		return
	}
	utils.LogInfo("Processing wallet topup payment simulation for order ID: %s, user ID: %d", orderIDStr, userID)

	// Convert order ID to uint
	var orderID uint
	if _, err := fmt.Sscanf(orderIDStr, "%d", &orderID); err != nil {
		utils.LogError("Invalid order ID format: %s", orderIDStr)
		utils.BadRequest(c, "Invalid order ID format", nil)
		return
	}

	// Get the wallet topup order from database
	db := config.DB
	var walletTopupOrder models.WalletTopupOrder
	if err := db.Where("id = ? AND user_id = ?", orderID, userID).First(&walletTopupOrder).Error; err != nil {
		utils.LogError("Wallet topup order not found for ID: %d, user ID: %d", orderID, userID)
		utils.NotFound(c, "Wallet topup order not found")
		return
	}

	// Check if order is still in "pending" status
	utils.LogInfo("Checking wallet topup order status for ID: %d, current status: %s", orderID, walletTopupOrder.Status)
	if walletTopupOrder.Status != "pending" {
		utils.LogError("Wallet topup order ID: %d is not in 'pending' status. Current status: %s", orderID, walletTopupOrder.Status)
		utils.BadRequest(c, "Payment already completed for this wallet topup order", nil)
		return
	}
	utils.LogInfo("Wallet topup order ID: %d is in 'pending' status, proceeding with payment simulation", orderID)

	// Use the actual Razorpay order ID from the database
	razorpayOrderID := walletTopupOrder.RazorpayOrderID
	if razorpayOrderID == "" {
		utils.LogError("No Razorpay order ID found for wallet topup order ID: %d", orderID)
		utils.BadRequest(c, "Payment not initiated for this wallet topup order", nil)
		return
	}

	utils.LogInfo("Found Razorpay order ID: %s for wallet topup order ID: %d", razorpayOrderID, orderID)

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
	utils.LogInfo("Wallet topup payment simulation completed successfully for order ID: %d", orderID)

	utils.Success(c, "Payment simulation completed successfully", gin.H{
		"razorpay_order_id":   razorpayOrderID,
		"razorpay_payment_id": paymentID,
		"razorpay_signature":  signature,
	})
}
