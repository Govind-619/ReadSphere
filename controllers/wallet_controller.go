package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetWalletBalance returns the user's wallet balance
func GetWalletBalance(c *gin.Context) {
	// Get user from context
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	user := userVal.(models.User)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"balance": fmt.Sprintf("%.2f", wallet.Balance),
	})
}

// GetWalletTransactions returns the user's wallet transactions
func GetWalletTransactions(c *gin.Context) {
	// Get user from context
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	user := userVal.(models.User)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count transactions"})
		return
	}

	if err := config.DB.Where("wallet_id = ?", wallet.ID).Order("created_at DESC").Limit(limit).Offset(offset).Find(&transactions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get transactions"})
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
			"created_at":  txn.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": formattedTransactions,
		"total":        total,
		"page":         page,
		"limit":        limit,
		"pages":        (total + int64(limit) - 1) / int64(limit),
	})
}

// DEPRECATED: ProcessOrderCancellation has been merged into CancelOrder in order_controller.go
// This function is kept for reference but should not be used
// Use controllers.CancelOrder instead for all order cancellations with wallet refunds
func ProcessOrderCancellation(c *gin.Context) {
	// Return a deprecation notice
	c.JSON(http.StatusGone, gin.H{
		"error": "This endpoint is deprecated. Please use /user/orders/:id/cancel instead.",
	})

	/* Original function code is commented out for reference
	// Get user from context
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}
	user := userVal.(models.User)

	// Parse order ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// Get the order
	var order models.Order
	if err := config.DB.Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if order can be cancelled
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusProcessing {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order cannot be cancelled at this stage"})
		return
	}

	// Parse cancellation reason
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reason is required"})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Update order status
	order.Status = models.OrderStatusCancelled
	order.CancellationReason = req.Reason
	order.RefundStatus = "pending"
	order.RefundAmount = order.FinalTotal
	order.RefundedToWallet = true

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	// Create a wallet transaction
	orderIDUint := uint(orderID)
	reference := fmt.Sprintf("REFUND-ORDER-%d", orderID)
	description := fmt.Sprintf("Refund for cancelled order #%d", orderID)

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
		"message": "Order cancelled and refunded to wallet",
		"order": gin.H{
			"id":            order.ID,
			"status":        order.Status,
			"refund_amount": order.RefundAmount,
			"refund_status": order.RefundStatus,
			"refunded_at":   order.RefundedAt,
		},
		"transaction": transaction,
		"wallet": gin.H{
			"balance": wallet.Balance,
		},
	})
	*/
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
