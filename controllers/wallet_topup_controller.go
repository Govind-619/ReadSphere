package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
)

// InitiateWalletTopup initiates a payment to add money to the wallet
func InitiateWalletTopup(c *gin.Context) {
	fmt.Println("InitiateWalletTopup endpoint called")
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	userID := user.ID

	var req struct {
		Amount float64 `json:"amount" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. Amount is required and must be positive", "details": err.Error()})
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	// Razorpay expects amount in paise
	amountPaise := int(req.Amount * 100)

	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY"), os.Getenv("RAZORPAY_SECRET"))
	orderData := map[string]interface{}{
		"amount":          amountPaise,
		"currency":        "INR",
		"receipt":         "wallet_topup_" + strconv.FormatUint(uint64(userID), 10) + "_" + time.Now().Format("20060102150405"),
		"payment_capture": 1,
	}
	rzOrder, err := client.Order.Create(orderData, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Razorpay order", "details": err.Error()})
		return
	}

	// Save WalletTopupOrder in DB
	walletTopupOrder := models.WalletTopupOrder{
		UserID: userID,
		RazorpayOrderID: fmt.Sprintf("%v", rzOrder["id"]),
		Amount: req.Amount,
		Status: "pending",
	}
	if err := config.DB.Create(&walletTopupOrder).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record wallet topup order", "details": err.Error()})
		return
	}


	c.JSON(http.StatusOK, gin.H{
		"razorpay_order_id": rzOrder["id"],
		"amount":            rzOrder["amount"],
		"amount_display":    "₹" + fmt.Sprintf("%.2f", float64(amountPaise)/100),
		"currency":          rzOrder["currency"],
		"key":               os.Getenv("RAZORPAY_KEY"),
		"user": gin.H{
			"name":  user.Username,
			"email": user.Email,
		},
		"wallet": gin.H{
			"id":      wallet.ID,
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		},
		"payment_type": "wallet_topup",
	})
}

// VerifyWalletTopup verifies the payment and adds money to the wallet
func VerifyWalletTopup(c *gin.Context) {
	fmt.Println("VerifyWalletTopup endpoint called")
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	userID := user.ID

	var req struct {
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Fetch the WalletTopupOrder from DB to get the amount
	var walletTopupOrder models.WalletTopupOrder
	err := config.DB.Where("razorpay_order_id = ?", req.RazorpayOrderID).First(&walletTopupOrder).Error
	if err != nil || walletTopupOrder.Amount <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unable to fetch wallet topup amount for this order_id"})
		return
	}
	amount := walletTopupOrder.Amount

	// Verify signature
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	generatedSignature := hex.EncodeToString(h.Sum(nil))
	if generatedSignature != req.RazorpaySignature {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failure", "message": "Payment verification failed", "retry": true})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(userID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	// Create a wallet transaction
	reference := fmt.Sprintf("TOPUP-%s", req.RazorpayPaymentID)
	description := fmt.Sprintf("Wallet topup via Razorpay")

	transaction, err := createWalletTransaction(wallet.ID, amount, models.TransactionTypeCredit, description, nil, reference)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	// Update wallet balance
	if err := updateWalletBalance(wallet.ID, amount, models.TransactionTypeCredit); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Get updated wallet
	updatedWallet, err := getOrCreateWallet(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get updated wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"message":          "Money added to wallet successfully!",
		"amount_added":     fmt.Sprintf("%.2f", amount),
		"wallet_balance":   fmt.Sprintf("%.2f", updatedWallet.Balance),
		"transaction_id":   transaction.ID,
		"transaction_date": transaction.CreatedAt,
		"reference":        reference,
	})
}
