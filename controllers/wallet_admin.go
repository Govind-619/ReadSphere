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

func ApproveOrderReturn(c *gin.Context) {
	utils.LogInfo("ApproveOrderReturn called")
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Parse order ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	utils.LogInfo("Processing return approval for order ID: %d", orderID)

	// Get the order
	var order models.Order
	if err := config.DB.Preload("User").Preload("OrderItems").Where("id = ?", orderID).First(&order).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if order status is return requested
	if order.Status != models.OrderStatusReturnRequested && order.Status != "Returned" {
		utils.LogError("Invalid order status for return approval - Order ID: %d, Status: %s", orderID, order.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not in return requested status"})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", orderID, tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}
	utils.LogDebug("Started transaction for order ID: %d", orderID)

	// Restock books
	for _, item := range order.OrderItems {
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to restock books for order ID: %d, Book ID: %d: %v", orderID, item.BookID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restock books"})
			return
		}
		utils.LogDebug("Restocked book ID: %d with quantity: %d", item.BookID, item.Quantity)
	}

	// Update order status
	order.Status = models.OrderStatusReturnApproved
	order.RefundStatus = "pending"
	order.RefundAmount = order.FinalTotal
	order.RefundedToWallet = true

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update order status - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}
	utils.LogDebug("Updated order status for order ID: %d", orderID)

	// Get or create wallet
	wallet, err := getOrCreateWallet(order.UserID)
	if err != nil {
		tx.Rollback()
		utils.LogError("Failed to get wallet for user ID: %d: %v", order.UserID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}
	utils.LogDebug("Retrieved wallet for user ID: %d", order.UserID)

	// Create a wallet transaction
	orderIDUint := uint(orderID)
	reference := fmt.Sprintf("REFUND-RETURN-%d", orderID)
	description := fmt.Sprintf("Refund for returned order #%d", orderID)

	transaction, err := createWalletTransaction(wallet.ID, order.FinalTotal, models.TransactionTypeCredit, description, &orderIDUint, reference)
	if err != nil {
		tx.Rollback()
		utils.LogError("Failed to create wallet transaction - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
		return
	}
	utils.LogDebug("Created wallet transaction for order ID: %d", orderID)

	// Update wallet balance
	if err := updateWalletBalance(wallet.ID, order.FinalTotal, models.TransactionTypeCredit); err != nil {
		tx.Rollback()
		utils.LogError("Failed to update wallet balance - Wallet ID: %d: %v", wallet.ID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
		return
	}
	utils.LogDebug("Updated wallet balance for wallet ID: %d", wallet.ID)

	// Update order refund status
	now := time.Now()
	order.RefundStatus = "completed"
	order.RefundedAt = &now
	order.Status = models.OrderStatusReturnCompleted

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update order refund status - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order refund status"})
		return
	}
	utils.LogDebug("Updated order refund status for order ID: %d", orderID)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}
	utils.LogInfo("Successfully approved return and processed refund for order ID: %d", orderID)

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
	utils.LogInfo("RejectOrderReturn called")
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Parse order ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	utils.LogInfo("Processing return rejection for order ID: %d", orderID)

	// Get the order
	var order models.Order
	if err := config.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if order status is valid for rejection
	if order.Status != models.OrderStatusReturnRequested && order.Status != "Returned" {
		utils.LogError("Invalid order status for rejection - Order ID: %d, Status: %s", orderID, order.Status)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not in a valid state for return rejection"})
		return
	}

	// Parse rejection reason
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Missing rejection reason for order ID: %d: %v", orderID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reason is required"})
		return
	}

	// Update order status
	order.Status = models.OrderStatusReturnRejected
	order.ReturnRejectReason = req.Reason
	order.UpdatedAt = time.Now()

	if err := config.DB.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order status - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}
	utils.LogInfo("Successfully rejected return request for order ID: %d", orderID)

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
	utils.LogDebug("Getting or creating wallet for user ID: %d", userID)
	var wallet models.Wallet
	err := config.DB.Where("user_id = ?", userID).First(&wallet).Error
	if err != nil {
		// Wallet doesn't exist, create one
		wallet = models.Wallet{
			UserID:  userID,
			Balance: 0,
		}
		if err := config.DB.Create(&wallet).Error; err != nil {
			utils.LogError("Failed to create wallet for user ID: %d: %v", userID, err)
			return nil, err
		}
		utils.LogDebug("Created new wallet for user ID: %d", userID)
	}
	return &wallet, nil
}

// Helper function to create a wallet transaction
func createWalletTransaction(walletID uint, amount float64, transactionType string, description string, orderID *uint, reference string) (*models.WalletTransaction, error) {
	utils.LogDebug("Creating wallet transaction - Wallet ID: %d, Amount: %.2f, Type: %s", walletID, amount, transactionType)
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
		utils.LogError("Failed to create wallet transaction - Wallet ID: %d: %v", walletID, err)
		return nil, err
	}
	utils.LogDebug("Created wallet transaction ID: %d", transaction.ID)
	return &transaction, nil
}

// Helper function to update wallet balance
func updateWalletBalance(walletID uint, amount float64, transactionType string) error {
	utils.LogDebug("Updating wallet balance - Wallet ID: %d, Amount: %.2f, Type: %s", walletID, amount, transactionType)
	var wallet models.Wallet
	if err := config.DB.First(&wallet, walletID).Error; err != nil {
		utils.LogError("Failed to get wallet - Wallet ID: %d: %v", walletID, err)
		return err
	}

	if transactionType == models.TransactionTypeCredit {
		wallet.Balance += amount
	} else if transactionType == models.TransactionTypeDebit {
		if wallet.Balance < amount {
			utils.LogError("Insufficient balance - Wallet ID: %d, Required: %.2f, Available: %.2f", walletID, amount, wallet.Balance)
			return fmt.Errorf("insufficient balance")
		}
		wallet.Balance -= amount
	}

	if err := config.DB.Save(&wallet).Error; err != nil {
		utils.LogError("Failed to save wallet balance - Wallet ID: %d: %v", walletID, err)
		return err
	}
	utils.LogDebug("Updated wallet balance - Wallet ID: %d, New Balance: %.2f", walletID, wallet.Balance)
	return nil
}
