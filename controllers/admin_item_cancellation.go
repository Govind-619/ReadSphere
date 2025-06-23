package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Govind-619/ReadSphere/utils"
)

// AdminReviewItemCancellation handles admin approval or rejection of item cancellation requests
func AdminReviewItemCancellation(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found")
		return
	}

	// Parse order ID and item ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}

	// Parse request body
	var req struct {
		Action string `json:"action" binding:"required"` // "approve" or "reject"
		Reason string `json:"reason"`                    // Required for rejection
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request data", err.Error())
		return
	}

	// Validate action
	if req.Action != "approve" && req.Action != "reject" {
		utils.BadRequest(c, "Action must be either 'approve' or 'reject'", nil)
		return
	}

	// If rejecting, reason is required
	if req.Action == "reject" && req.Reason == "" {
		utils.BadRequest(c, "Reason is required when rejecting a cancellation request", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	// Get the order item
	var item models.OrderItem
	if err := tx.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Order item not found"})
		return
	}

	// Get the order
	var order models.Order
	if err := tx.Preload("User").First(&order, orderID).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	// Check if the item has a cancellation request
	if !item.CancellationRequested {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "This item does not have a pending cancellation request"})
		return
	}

	// Process based on action
	if req.Action == "approve" {
		// Update item status
		item.CancellationStatus = "Approved"

		// Restore stock for this item
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore book stock"})
			return
		}

		// If payment was made with wallet or online payment, process refund
		shouldRefundWallet := order.PaymentMethod == "wallet" || order.PaymentMethod == "RAZORPAY" || order.PaymentMethod == "online"

		if shouldRefundWallet {
			// Get or create wallet
			wallet, err := getOrCreateWallet(order.UserID)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
				return
			}

			// Calculate refund amount for this item
			// Use the final price the customer actually paid for this item
			refundAmount := item.Total - item.CouponDiscount // This is the final price after all discounts

			// Create a wallet transaction
			reference := fmt.Sprintf("REFUND-ORDER-%d-ITEM-%d", orderID, itemID)
			description := fmt.Sprintf("Refund for cancelled item #%d in order #%d", itemID, orderID)
			orderIDUint := uint(orderID)

			_, err = createWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &orderIDUint, reference)
			if err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet transaction"})
				return
			}

			// Update wallet balance
			if err := updateWalletBalance(wallet.ID, refundAmount, models.TransactionTypeCredit); err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
				return
			}
		}

		// Update order totals
		order.TotalAmount -= item.Total
		order.FinalTotal -= item.Total

		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order totals"})
			return
		}

		// Save the item with updated status
		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}

		// Check if this was the last pending item cancellation request
		var pendingRequests int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND cancellation_requested = ? AND cancellation_status = ?", orderID, true, "Pending").Count(&pendingRequests)

		if pendingRequests == 0 {
			order.HasItemCancellationRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
		}

		// Get the wallet transaction for reference
		var transaction models.WalletTransaction
		transactionRef := fmt.Sprintf("REFUND-RETURN-ORDER-%d-ITEM-%d", orderID, itemID)
		if err := tx.Where("reference = ?", transactionRef).First(&transaction).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet transaction"})
			return
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete approval"})
			return
		}

		utils.Success(c, "Item cancellation approved and processed", gin.H{
			"item": gin.H{
				"id":                  item.ID,
				"cancellation_status": "Approved",
			},
			"order": gin.H{
				"id":            order.ID,
				"updated_total": fmt.Sprintf("%.2f", order.FinalTotal),
			},
		})
	} else {
		// Reject the cancellation request
		item.CancellationStatus = "Rejected"
		item.CancellationReason = req.Reason

		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}

		// Check if this was the last pending item cancellation request
		var pendingRequests int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND cancellation_requested = ? AND cancellation_status = ?", orderID, true, "Pending").Count(&pendingRequests)

		if pendingRequests == 0 {
			order.HasItemCancellationRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete rejection"})
			return
		}

		utils.Success(c, "Item cancellation rejected", gin.H{
			"item": gin.H{
				"id":                  item.ID,
				"cancellation_status": "Rejected",
				"reject_reason":       req.Reason,
			},
		})
	}
}
