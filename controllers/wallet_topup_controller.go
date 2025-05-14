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
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID

	var req struct {
		Amount float64 `json:"amount" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request. Amount is required and must be positive", err.Error())
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(userID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
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
		utils.InternalServerError(c, "Failed to create Razorpay order", err.Error())
		return
	}

	// Save WalletTopupOrder in DB
	walletTopupOrder := models.WalletTopupOrder{
		UserID:          userID,
		RazorpayOrderID: fmt.Sprintf("%v", rzOrder["id"]),
		Amount:          req.Amount,
		Status:          "pending",
	}
	if err := config.DB.Create(&walletTopupOrder).Error; err != nil {
		utils.InternalServerError(c, "Failed to record wallet topup order", err.Error())
		return
	}

	utils.Success(c, "Wallet topup order created successfully", gin.H{
		"razorpay_order_id": rzOrder["id"],
		"amount_display":    "₹" + fmt.Sprintf("%.2f", float64(amountPaise)/100),
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
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID

	var req struct {
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Fetch the WalletTopupOrder from DB to get the amount
	var walletTopupOrder models.WalletTopupOrder
	err := config.DB.Where("razorpay_order_id = ?", req.RazorpayOrderID).First(&walletTopupOrder).Error
	if err != nil || walletTopupOrder.Amount <= 0 {
		utils.BadRequest(c, "Unable to fetch wallet topup amount for this order_id", nil)
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
		utils.BadRequest(c, "Payment verification failed", gin.H{"retry": true})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", tx.Error.Error())
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(userID)
	if err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}

	// Create a wallet transaction
	reference := fmt.Sprintf("TOPUP-%s", req.RazorpayPaymentID)
	description := "Wallet topup via Razorpay"

	transaction, err := createWalletTransaction(wallet.ID, amount, models.TransactionTypeCredit, description, nil, reference)
	if err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create transaction", err.Error())
		return
	}

	// Update wallet balance
	if err := updateWalletBalance(wallet.ID, amount, models.TransactionTypeCredit); err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update wallet balance", err.Error())
		return
	}

	// Update wallet topup order status
	walletTopupOrder.Status = "completed"
	if err := tx.Save(&walletTopupOrder).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update topup order status", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	// Get updated wallet
	updatedWallet, err := getOrCreateWallet(userID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get updated wallet", err.Error())
		return
	}

	utils.Success(c, "Money added to wallet successfully!", gin.H{
		"amount_added":     fmt.Sprintf("%.2f", amount),
		"wallet_balance":   fmt.Sprintf("%.2f", updatedWallet.Balance),
		"transaction_id":   transaction.ID,
		"transaction_date": transaction.CreatedAt.Format("2006-01-02 15:04:05"),
		"reference":        reference,
	})
}
