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

// AdminListReturnItems lists all item return requests including processed ones
func AdminListReturnItems(c *gin.Context) {
	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found")
		return
	}

	// Query all orders with return requests
	var orders []models.Order
	query := config.DB.Preload("User").
		Preload("OrderItems.Book").
		Preload("OrderItems", func(db *gorm.DB) *gorm.DB {
			return db.Where("return_requested = ?", true)
		}).
		Where("has_item_return_requests = ? OR EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND return_requested = true)", true)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	// First, get total count of return items
	var total int64
	config.DB.Model(&models.OrderItem{}).
		Where("return_requested = ?", true).
		Count(&total)

	// Get all orders but we'll paginate the items
	if err := query.Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}

	// Prepare response with return details
	var requests []gin.H
	itemCount := 0
	startIdx := (page - 1) * limit

	// Process orders and collect items with pagination
	for _, order := range orders {
		for _, item := range order.OrderItems {
			if item.ReturnRequested {
				// Skip items before the start index
				if itemCount < startIdx {
					itemCount++
					continue
				}
				// Break if we've reached our limit
				if len(requests) >= limit {
					break
				}

				// Initialize base request object
				req := gin.H{
					"order_id":     order.ID,
					"username":     order.User.Username,
					"email":        order.User.Email,
					"item_id":      item.ID,
					"book_name":    item.Book.Name,
					"quantity":     item.Quantity,
					"total":        item.Total,
					"reason":       item.ReturnReason,
					"status":       item.ReturnStatus,
					"requested_at": order.UpdatedAt.Format("2006-01-02 15:04:05"),
				}

				// Set default refund status
				req["refund_status"] = "pending"
				req["refund_amount"] = 0.0

				// Handle different return statuses
				switch item.ReturnStatus {
				case "Approved":
					if item.RefundedAt != nil {
						req["processed_at"] = item.RefundedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "completed"
						req["refund_amount"] = item.RefundAmount
					} else {
						req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "processing"
					}
				case "Rejected":
					req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
					req["reject_reason"] = item.ReturnReason
					req["refund_status"] = "rejected"
				case "Pending":
					req["refund_status"] = "pending"
				}

				requests = append(requests, req)
				itemCount++
			}
		}
		if len(requests) >= limit {
			break
		}
	}

	utils.Success(c, "Return requests retrieved successfully", gin.H{
		"requests": requests,
		"pagination": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
			"has_more":    (int64(page)*int64(limit) < total),
		},
	})
}

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
			itemTotal := item.Total

			// Calculate equal coupon discount per item
			var couponDiscountPerItem float64
			if order.CouponDiscount > 0 && len(order.OrderItems) > 0 {
				// Distribute coupon discount equally among all items
				couponDiscountPerItem = order.CouponDiscount / float64(len(order.OrderItems))
			}

			// Final refund amount is item total minus equal coupon discount per item
			refundAmount := itemTotal - couponDiscountPerItem

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

// AdminReviewReturnItem handles admin approval or rejection of item return requests
func AdminReviewReturnItem(c *gin.Context) {
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
		Action  string `json:"action" binding:"required"` // "approve" or "reject"
		Reason  string `json:"reason"`                    // Required for rejection
		Quality string `json:"quality"`                   // Required for approval
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
		utils.BadRequest(c, "Reason is required when rejecting a return request", nil)
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

	// Check if the item has a return request
	if !item.ReturnRequested || item.ReturnStatus != "Pending" {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "This item does not have a pending return request"})
		return
	}

	if req.Action == "approve" {
		// Update item status
		item.ReturnStatus = "Approved"
		item.RefundStatus = "processing" // Set initial refund status

		// Process refund
		wallet, err := getOrCreateWallet(order.UserID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
			return
		}

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

		// Create wallet transaction
		transactionRef := fmt.Sprintf("REFUND-RETURN-ORDER-%d-ITEM-%d", orderID, itemID)
		description := fmt.Sprintf("Refund for returned item #%d in order #%d", itemID, orderID)
		orderIDUint := uint(orderID)

		_, err = createWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &orderIDUint, transactionRef)
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

		// Update refund status after successful transaction
		item.RefundStatus = "completed"
		item.RefundAmount = refundAmount

		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}

		// Check if this was the last pending return request
		var pendingReturns int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND return_requested = ? AND return_status = ?", orderID, true, "Pending").Count(&pendingReturns)

		if pendingReturns == 0 {
			order.HasItemReturnRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
		}

		// Get the wallet transaction for reference
		var transaction models.WalletTransaction
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
		// Reject the return request
		item.ReturnStatus = "Rejected"
		item.ReturnReason = req.Reason
		item.RefundStatus = "rejected" // Set refund status for rejected returns

		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update item status"})
			return
		}

		// Check if this was the last pending return request
		var pendingReturns int64
		tx.Model(&models.OrderItem{}).Where("order_id = ? AND return_requested = ? AND return_status = ?", orderID, true, "Pending").Count(&pendingReturns)

		if pendingReturns == 0 {
			order.HasItemReturnRequests = false
			if err := tx.Save(&order).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order status"})
				return
			}
		}

		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete rejection"})
			return
		}

		utils.Success(c, "Return request rejected", gin.H{
			"item": gin.H{
				"id":            item.ID,
				"return_status": "Rejected",
				"reject_reason": req.Reason,
			},
		})
	}
}
