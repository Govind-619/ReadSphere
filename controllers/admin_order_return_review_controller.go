package controllers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminReviewReturn handles the admin review of return requests
func AdminReviewReturn(c *gin.Context) {
	// Verify admin
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}
	_, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	// Parse parameters
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil {
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}

	var req struct {
		Action  string `json:"action" binding:"required,oneof=approve reject"`
		Reason  string `json:"reason,omitempty"`
		Quality string `json:"quality,omitempty" binding:"omitempty,oneof=good damaged unusable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request body", err.Error())
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	// Get order with items and user
	var order models.Order
	if err := tx.Preload("OrderItems").Preload("User").First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Order not found")
		return
	}

	// Find the specific item
	var item models.OrderItem
	found := false
	for _, i := range order.OrderItems {
		if i.ID == uint(itemID) {
			item = i
			found = true
			break
		}
	}

	if !found {
		tx.Rollback()
		utils.NotFound(c, "Order item not found")
		return
	}

	// Verify item has a pending return request
	if !item.ReturnRequested || item.ReturnStatus != models.OrderStatusReturnRequested {
		tx.Rollback()
		utils.BadRequest(c, "No pending return request found for this item", nil)
		return
	}

	switch req.Action {
	case "approve":
		// Update item status
		item.ReturnStatus = models.OrderStatusReturnApproved

		// Restore stock if item quality is good
		if req.Quality == "good" {
			if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).
				UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				tx.Rollback()
				utils.InternalServerError(c, "Failed to restore book stock", nil)
				return
			}
		}

		// Process refund
		refundAmount := item.Total
		wallet, err := utils.GetOrCreateWallet(order.UserID)
		if err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to process refund", nil)
			return
		}

		// Create refund transaction
		reference := fmt.Sprintf("REFUND-RETURN-ORDER-%d-ITEM-%d", orderID, itemID)
		description := fmt.Sprintf("Refund for returned item in order #%d", orderID)

		if _, err := utils.CreateWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &order.ID, reference); err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to create refund transaction", nil)
			return
		}

		// Update wallet balance
		if err := utils.UpdateWalletBalance(wallet.ID, refundAmount, models.TransactionTypeCredit); err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update wallet balance", nil)
			return
		}

		// Update refund status
		now := time.Now()
		item.RefundStatus = "completed"
		item.RefundAmount = refundAmount
		item.RefundedAt = &now

	case "reject":
		if req.Reason == "" {
			tx.Rollback()
			utils.BadRequest(c, "Reason is required when rejecting a return", nil)
			return
		}
		item.ReturnStatus = models.OrderStatusReturnRejected
		order.ReturnRejectReason = req.Reason
	}

	// Save item changes
	if err := tx.Save(&item).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}

	// Update order status if all returns are processed
	var pendingReturns int
	for _, i := range order.OrderItems {
		if i.ReturnStatus == models.OrderStatusReturnRequested {
			pendingReturns++
		}
	}

	if pendingReturns == 0 {
		order.HasItemReturnRequests = false
		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update order status", nil)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to process return review", nil)
		return
	}

	utils.Success(c, fmt.Sprintf("Return request %s successfully", req.Action), gin.H{
		"item": gin.H{
			"id":            item.ID,
			"return_status": item.ReturnStatus,
			"refund_status": item.RefundStatus,
			"refund_amount": fmt.Sprintf("%.2f", item.RefundAmount),
		},
		"order": gin.H{
			"id":                    order.ID,
			"has_pending_returns":   pendingReturns > 0,
			"pending_returns_count": pendingReturns,
		},
	})
}
