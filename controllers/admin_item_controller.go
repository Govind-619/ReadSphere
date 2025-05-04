package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminListItemCancellationRequests lists all item cancellation requests pending admin action
func AdminListItemCancellationRequests(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Query all orders with pending item cancellation requests
	var orders []models.Order
	query := config.DB.Preload("User").Preload("OrderItems", "cancellation_requested = ?", true).
		Where("has_item_cancellation_requests = ?", true)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	var total int64
	query.Model(&models.Order{}).Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Find(&orders)

	// Prepare response
	type CancellationRequest struct {
		OrderID     uint   `json:"order_id"`
		Username    string `json:"username"`
		Email       string `json:"email"`
		ItemID      uint   `json:"item_id"`
		BookName    string `json:"book_name"`
		Reason      string `json:"reason"`
		RequestedAt string `json:"requested_at"`
	}

	var requests []CancellationRequest
	for _, order := range orders {
		for _, item := range order.OrderItems {
			if item.CancellationRequested && item.CancellationStatus == "Pending" {
				// Load book details
				var book models.Book
				config.DB.First(&book, item.BookID)

				requests = append(requests, CancellationRequest{
					OrderID:     order.ID,
					Username:    order.User.Username,
					Email:       order.User.Email,
					ItemID:      item.ID,
					BookName:    book.Name,
					Reason:      item.CancellationReason,
					RequestedAt: order.UpdatedAt.Format("2006-01-02 15:04:05"),
				})
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// AdminReviewItemCancellation handles admin approval or rejection of item cancellation requests
func AdminReviewItemCancellation(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
		return
	}

	// Parse order ID and item ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	itemID, err := strconv.ParseUint(c.Param("item_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}

	// Parse request body
	var req struct {
		Action string `json:"action" binding:"required"` // "approve" or "reject"
		Reason string `json:"reason"`                    // Required for rejection
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	// Validate action
	if req.Action != "approve" && req.Action != "reject" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Action must be either 'approve' or 'reject'"})
		return
	}

	// If rejecting, reason is required
	if req.Action == "reject" && req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Reason is required when rejecting a cancellation request"})
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
			refundAmount := item.Total

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

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete approval"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Item cancellation approved and processed",
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

		c.JSON(http.StatusOK, gin.H{
			"message": "Item cancellation rejected",
			"reason":  req.Reason,
			"item": gin.H{
				"id":                  item.ID,
				"cancellation_status": "Rejected",
			},
		})
	}
}
