package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AdminReviewReturnItem handles admin approval or rejection of item return requests
func AdminReviewReturnItem(c *gin.Context) {
	utils.LogInfo("AdminReviewReturnItem called")

	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		utils.Unauthorized(c, "Admin not found")
		return
	}

	// Parse order ID and item ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid order ID: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid item ID: %v", err)
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}
	utils.LogDebug("Processing return request for order %d, item %d", orderID, itemID)

	// Parse request body
	var req struct {
		Action  string `json:"action" binding:"required"` // "approve" or "reject"
		Reason  string `json:"reason"`                    // Required for rejection
		Quality string `json:"quality"`                   // Required for approval
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request data: %v", err)
		utils.BadRequest(c, "Invalid request data", err.Error())
		return
	}
	utils.LogDebug("Request action: %s, Reason: %s, Quality: %s", req.Action, req.Reason, req.Quality)

	// Validate action
	if req.Action != "approve" && req.Action != "reject" {
		utils.LogError("Invalid action: %s", req.Action)
		utils.BadRequest(c, "Action must be either 'approve' or 'reject'", nil)
		return
	}

	// If rejecting, reason is required
	if req.Action == "reject" && req.Reason == "" {
		utils.LogError("Missing reason for rejection")
		utils.BadRequest(c, "Reason is required when rejecting a return request", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction: %v", tx.Error)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}
	utils.LogDebug("Started database transaction")

	// Get the order item
	var item models.OrderItem
	if err := tx.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		tx.Rollback()
		utils.LogError("Order item not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order item not found"})
		return
	}
	utils.LogDebug("Found order item with status: %s", item.ReturnStatus)

	// Get the order
	var order models.Order
	if err := tx.Preload("User").First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.LogError("Order not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	utils.LogDebug("Found order for user: %s", order.User.Username)

	// Check if the item has a return request
	if !item.ReturnRequested || item.ReturnStatus != "Pending" {
		tx.Rollback()
		utils.LogError("Item does not have a pending return request")
		c.JSON(http.StatusBadRequest, gin.H{"error": "This item does not have a pending return request"})
		return
	}

	if req.Action == "approve" {
		utils.LogDebug("Processing approval for item %d", itemID)
		// Update item status
		item.ReturnStatus = "Approved"
		item.RefundStatus = "processing" // Set initial refund status

		// Process refund
		wallet, err := getOrCreateWallet(order.UserID)
		if err != nil {
			tx.Rollback()
			utils.LogError("Failed to get wallet: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
			return
		}
		utils.LogDebug("Found wallet for user %d", order.UserID)

		// Calculate refund amount for this item
		itemTotal := item.Total

		// Calculate equal coupon discount per item
		var couponDiscountPerItem float64
		if order.CouponDiscount > 0 && len(order.OrderItems) > 0 {
			// Distribute coupon discount equally among all items
			couponDiscountPerItem = order.CouponDiscount / float64(len(order.OrderItems))
		}

		// Final refund amount is item total minus equal coupon discount per item
		refundAmount := itemTotal - couponDiscountPerItem
		utils.LogDebug("Calculated refund amount: %.2f (Item total: %.2f, Coupon discount per item: %.2f)",
			refundAmount, itemTotal, couponDiscountPerItem)

		// Create wallet transaction
		transactionRef := fmt.Sprintf("REFUND-RETURN-ORDER-%d-ITEM-%d", orderID, itemID)
		description := fmt.Sprintf("Refund for returned item #%d in order #%d", itemID, orderID)
		orderIDUint := uint(orderID)

		_, err = createWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &orderIDUint, transactionRef)
		if err != nil {
			tx.Rollback()
			utils.LogError("Failed to create wallet transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create wallet transaction"})
			return
		}
		utils.LogDebug("Created wallet transaction with reference: %s", transactionRef)

		// Update wallet balance
		if err := updateWalletBalance(wallet.ID, refundAmount, models.TransactionTypeCredit); err != nil {
			tx.Rollback()
			utils.LogError("Failed to update wallet balance: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
			return
		}
		utils.LogDebug("Updated wallet balance for user %d", order.UserID)

		// Update refund status after successful transaction
		item.RefundStatus = "completed"
		item.RefundAmount = refundAmount

		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to update item status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}
		utils.LogDebug("Updated item status to approved and refund completed")

		// Check if this was the last pending return request
		var pendingReturns int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND return_requested = ? AND return_status = ?", orderID, true, "Pending").Count(&pendingReturns)

		if pendingReturns == 0 {
			order.HasItemReturnRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				utils.LogError("Failed to update order status: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
			utils.LogDebug("Updated order status - no more pending returns")
		}

		// Get the wallet transaction for reference
		var transaction models.WalletTransaction
		if err := tx.Where("reference = ?", transactionRef).First(&transaction).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to fetch wallet transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch wallet transaction"})
			return
		}
		utils.LogDebug("Retrieved wallet transaction details")

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			utils.LogError("Failed to commit transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete approval"})
			return
		}
		utils.LogDebug("Successfully committed transaction")

		utils.LogInfo("Successfully approved return request for order %d, item %d", orderID, itemID)
		utils.Success(c, "Return request approved and refund processed", gin.H{
			"item": gin.H{
				"id":            item.ID,
				"return_status": "Approved",
				"refund_details": gin.H{
					"amount":       fmt.Sprintf("%.2f", refundAmount),
					"item_total":   fmt.Sprintf("%.2f", item.Total),
					"refunded_to":  "wallet",
					"refunded_at":  transaction.CreatedAt.Format("2006-01-02 15:04:05"),
					"item_quality": req.Quality,
					"transaction": gin.H{
						"id":          transaction.ID,
						"amount":      fmt.Sprintf("%.2f", transaction.Amount),
						"type":        transaction.Type,
						"description": transaction.Description,
						"created_at":  transaction.CreatedAt.Format("2006-01-02 15:04:05"),
					},
				},
			},
			"order": gin.H{
				"id": order.ID,
			},
		})
	} else {
		utils.LogDebug("Processing rejection for item %d", itemID)
		// Reject the return request
		item.ReturnStatus = "Rejected"
		item.ReturnReason = req.Reason
		item.RefundStatus = "rejected" // Set refund status for rejected returns

		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to update item status: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}
		utils.LogDebug("Updated item status to rejected")

		// Check if this was the last pending return request
		var pendingReturns int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND return_requested = ? AND return_status = ?", orderID, true, "Pending").Count(&pendingReturns)

		if pendingReturns == 0 {
			order.HasItemReturnRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				utils.LogError("Failed to update order status: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
			utils.LogDebug("Updated order status - no more pending returns")
		}

		if err := tx.Commit().Error; err != nil {
			utils.LogError("Failed to commit transaction: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete rejection"})
			return
		}
		utils.LogDebug("Successfully committed transaction")

		utils.LogInfo("Successfully rejected return request for order %d, item %d", orderID, itemID)
		utils.Success(c, "Return request rejected", gin.H{
			"item": gin.H{
				"id":            item.ID,
				"return_status": "Rejected",
				"reject_reason": req.Reason,
			},
		})
	}
}
