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

// GetWalletBalance returns the user's wallet balance
func GetWalletBalance(c *gin.Context) {
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

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}

	utils.Success(c, "Wallet balance retrieved successfully", gin.H{
		"balance": fmt.Sprintf("%.2f", wallet.Balance),
	})
}

// GetWalletTransactions returns the user's wallet transactions
func GetWalletTransactions(c *gin.Context) {
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

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get wallet", err.Error())
		return
	}

	// Get pagination params
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	offset := (page - 1) * limit

	// Get transactions
	var transactions []models.WalletTransaction
	var total int64
	if err := config.DB.Model(&models.WalletTransaction{}).Where("wallet_id = ?", wallet.ID).Count(&total).Error; err != nil {
		utils.InternalServerError(c, "Failed to count transactions", err.Error())
		return
	}

	if err := config.DB.Where("wallet_id = ?", wallet.ID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		utils.InternalServerError(c, "Failed to get transactions", err.Error())
		return
	}

	// Format transaction amounts
	formattedTransactions := make([]gin.H, len(transactions))
	for i, txn := range transactions {
		formattedTransactions[i] = gin.H{
			"id":          txn.ID,
			"amount":      fmt.Sprintf("%.2f", txn.Amount),
			"type":        txn.Type,
			"description": txn.Description,
			"reference":   txn.Reference,
			"created_at":  txn.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	utils.SuccessWithPagination(c, "Wallet transactions retrieved successfully", gin.H{
		"transactions": formattedTransactions,
		"wallet": gin.H{
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		},
	}, total, page, limit)
}

// ProcessOrderCancellation has been deprecated and merged into CancelOrder
func ProcessOrderCancellation(c *gin.Context) {
	utils.BadRequest(c, "This endpoint is deprecated. Please use /user/orders/:id/cancel instead", nil)
}

// Admin endpoint to approve return and process refund
func ApproveOrderReturn(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Parse order ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Get the order
	var order models.Order
	if err := config.DB.Preload("User").Preload("OrderItems").Where("id = ?", orderID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if order status is return requested
	if order.Status != models.OrderStatusReturnRequested && order.Status != "Returned" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not in return requested status"})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Restock books - added from AdminAcceptReturn
	for _, item := range order.OrderItems {
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restock books"})
			return
		}
	}

	// Update order status
	order.Status = models.OrderStatusReturnApproved
	order.RefundStatus = "pending"
	order.RefundAmount = order.FinalTotal
	order.RefundedToWallet = true

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(order.UserID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	// Create a wallet transaction
	orderIDUint := uint(orderID)
	reference := fmt.Sprintf("REFUND-RETURN-%d", orderID)
	description := fmt.Sprintf("Refund for returned order #%d", orderID)

	transaction, err := createWalletTransaction(wallet.ID, order.FinalTotal, models.TransactionTypeCredit, description, &orderIDUint, reference)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}

	// Update wallet balance
	if err := updateWalletBalance(wallet.ID, order.FinalTotal, models.TransactionTypeCredit); err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
		return
	}

	// Update order refund status
	now := time.Now()
	order.RefundStatus = "completed"
	order.RefundedAt = &now
	order.Status = models.OrderStatusReturnCompleted

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order refund status"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Return approved and refunded to wallet",
		"order": gin.H{
			"id":            order.ID,
			"status":        order.Status,
			"refund_amount": fmt.Sprintf("%.2f", order.RefundAmount),
			"refund_status": order.RefundStatus,
			"refunded_at":   order.RefundedAt,
		},
		"transaction": transaction,
	})
}

// Admin endpoint to reject a return request
func RejectOrderReturn(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Parse order ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Get the order
	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if order status is valid for rejection
	// Support both new ReturnRequested status and legacy Returned status
	if order.Status != models.OrderStatusReturnRequested && order.Status != "Returned" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not in a valid state for return rejection"})
		return
	}

	// Parse rejection reason
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reason is required"})
		return
	}

	// Update order status
	order.Status = models.OrderStatusReturnRejected
	order.ReturnRejectReason = req.Reason
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Return request rejected",
		"order": gin.H{
			"id":                   order.ID,
			"status":               order.Status,
			"return_reject_reason": order.ReturnRejectReason,
		},
	})
}

// Helper function to get or create a wallet for a user
func getOrCreateWallet(userID uint) (*models.Wallet, error) {
	var wallet models.Wallet
	err := config.DB.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		// Wallet doesn't exist, create one
		wallet = models.Wallet{
			UserID:  userID,
			Balance: 0,
		}
		if err := config.DB.Create(&wallet).Error; err != nil {
			return nil, err
		}
	}
	return &wallet, nil
}

// Helper function to create a wallet transaction
func createWalletTransaction(walletID uint, amount float64, transactionType string, description string, orderID *uint, reference string) (*models.WalletTransaction, error) {
	transaction := models.WalletTransaction{
		WalletID:    walletID,
		Amount:      amount,
		Type:        transactionType,
		Description: description,
		OrderID:     orderID,
		Reference:   reference,
		Status:      models.TransactionStatusCompleted,
	}

	if err := config.DB.Create(&transaction).Error; err != nil {
		return nil, err
	}

	return &transaction, nil
}

// Helper function to update wallet balance
func updateWalletBalance(walletID uint, amount float64, transactionType string) error {
	var wallet models.Wallet
	if err := config.DB.First(&wallet, walletID).Error; err != nil {
		return err
	}

	if transactionType == models.TransactionTypeCredit {
		wallet.Balance += amount
	} else if transactionType == models.TransactionTypeDebit {
		if wallet.Balance < amount {
			return fmt.Errorf("insufficient balance")
		}
		wallet.Balance -= amount
	}

	if err := config.DB.Save(&wallet).Error; err != nil {
		return err
	}

	return nil
}
