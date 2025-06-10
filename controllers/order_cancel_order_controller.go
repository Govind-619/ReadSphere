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

// CancelOrder cancels an entire order
func CancelOrder(c *gin.Context) {
	utils.LogInfo("CancelOrder called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing order cancellation for user ID: %d", user.ID)

	// Parse order ID
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogDebug("Processing cancellation for order ID: %d", orderID)

	// Parse cancellation reason
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Missing cancellation reason for order ID: %d: %v", orderID, err)
		utils.BadRequest(c, "Reason is required", nil)
		return
	}
	utils.LogDebug("Cancellation reason received for order ID: %d", orderID)

	// Get the order with all items
	var order models.Order
	if err := config.DB.Preload("OrderItems").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d, User ID: %d: %v", orderID, user.ID, err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order ID: %d with %d items", orderID, len(order.OrderItems))

	// Check if order is already cancelled
	if order.Status == models.OrderStatusCancelled {
		utils.LogError("Order already cancelled - Order ID: %d", orderID)
		utils.BadRequest(c, "Order already cancelled", nil)
		return
	}

	// Check if order can be cancelled based on status and time
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusProcessing {
		utils.LogError("Order cannot be cancelled - Order ID: %d, Status: %s", orderID, order.Status)
		utils.BadRequest(c, "Order cannot be cancelled at this stage", nil)
		return
	}

	// Check 30-minute cancellation window
	if time.Since(order.CreatedAt) > 30*time.Minute {
		utils.LogError("Cancellation window expired - Order ID: %d, Created: %v", orderID, order.CreatedAt)
		utils.BadRequest(c, "Cancellation window (30 minutes) has expired", nil)
		return
	}
	utils.LogDebug("Order within cancellation window - Order ID: %d", orderID)

	// Start a database transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", orderID, tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}
	utils.LogDebug("Started transaction for order cancellation - Order ID: %d", orderID)

	// Restore stock for each book
	for _, item := range order.OrderItems {
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
			utils.LogError("Failed to restore stock for book ID: %d, order ID: %d: %v", item.BookID, orderID, err)
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore book stock"})
			return
		}
		utils.LogDebug("Restored stock for book ID: %d, quantity: %d", item.BookID, item.Quantity)
	}

	// Update order status and details
	order.Status = models.OrderStatusCancelled
	order.CancellationReason = req.Reason
	order.RefundStatus = "pending"
	order.RefundAmount = order.FinalTotal
	order.RefundedToWallet = true
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order status - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}
	utils.LogDebug("Updated order status to cancelled - Order ID: %d", orderID)

	// Only process refund if payment was not COD
	var walletRefundProcessed bool
	var wallet *models.Wallet
	var transaction *models.WalletTransaction

	if order.PaymentMethod != "COD" && order.PaymentMethod != "cod" {
		utils.LogDebug("Processing refund for non-COD order - Order ID: %d, Payment Method: %s", orderID, order.PaymentMethod)
		// Get or create wallet
		wallet, err = utils.GetOrCreateWallet(user.ID)
		if err != nil {
			utils.LogError("Failed to get wallet for user ID: %d, order ID: %d: %v", user.ID, orderID, err)
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
			return
		}

		// Create a wallet transaction
		orderIDUint := uint(orderID)
		reference := fmt.Sprintf("REFUND-ORDER-%d", orderID)
		description := fmt.Sprintf("Refund for cancelled order #%d", orderID)

		transaction, err = utils.CreateWalletTransaction(wallet.ID, order.FinalTotal, models.TransactionTypeCredit, description, &orderIDUint, reference)
		if err != nil {
			utils.LogError("Failed to create wallet transaction - Order ID: %d: %v", orderID, err)
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
			return
		}
		utils.LogDebug("Created wallet transaction - Transaction ID: %d, Amount: %.2f", transaction.ID, transaction.Amount)

		// Update wallet balance
		if err := utils.UpdateWalletBalance(wallet.ID, order.FinalTotal, models.TransactionTypeCredit); err != nil {
			utils.LogError("Failed to update wallet balance - Wallet ID: %d, Order ID: %d: %v", wallet.ID, orderID, err)
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
			return
		}
		utils.LogDebug("Updated wallet balance - Wallet ID: %d, Amount: %.2f", wallet.ID, order.FinalTotal)

		// Update order refund status
		now := time.Now()
		order.RefundStatus = "completed"
		order.RefundedAt = &now

		if err := tx.Save(&order).Error; err != nil {
			utils.LogError("Failed to update order refund status - Order ID: %d: %v", orderID, err)
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order refund status"})
			return
		}
		utils.LogDebug("Updated order refund status to completed - Order ID: %d", orderID)

		walletRefundProcessed = true
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d", orderID)

	// Prepare response based on whether wallet refund was processed
	if order.PaymentMethod == "COD" || order.PaymentMethod == "cod" {
		utils.LogInfo("Order cancelled successfully (COD) - Order ID: %d", orderID)
		c.JSON(http.StatusOK, gin.H{
			"message": "Order cancelled",
			"order": gin.H{
				"id":            order.ID,
				"status":        order.Status,
				"refund_status": "No refund applicable for COD orders",
			},
		})
	} else if walletRefundProcessed {
		utils.LogInfo("Order cancelled and refunded to wallet - Order ID: %d, Amount: %.2f", orderID, order.RefundAmount)
		c.JSON(http.StatusOK, gin.H{
			"message": "Order cancelled and refunded to wallet",
			"order": gin.H{
				"id":            order.ID,
				"status":        order.Status,
				"refund_amount": fmt.Sprintf("%.2f", order.RefundAmount),
				"refund_status": order.RefundStatus,
				"refunded_at":   order.RefundedAt.Format("2006-01-02 15:04:05"),
			},
			"transaction": gin.H{
				"id":          transaction.ID,
				"wallet_id":   transaction.WalletID,
				"amount":      fmt.Sprintf("%.2f", transaction.Amount),
				"type":        transaction.Type,
				"description": transaction.Description,
				"order_id":    transaction.OrderID,
				"reference":   transaction.Reference,
				"status":      "success",
			},
			"wallet": gin.H{
				"balance": fmt.Sprintf("%.2f", wallet.Balance),
			},
		})
	}
}
