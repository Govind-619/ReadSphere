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

// CancelOrderItem cancels a single item in an order within 30 minutes of ordering
func CancelOrderItem(c *gin.Context) {
	utils.LogInfo("CancelOrderItem called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing item cancellation for user ID: %d", user.ID)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogDebug("Processing cancellation for order ID: %d", orderID)

	itemIDStr := c.Param("item_id")
	var itemID uint
	_, err = fmt.Sscanf(itemIDStr, "%d", &itemID)
	if err != nil || itemID == 0 {
		utils.LogError("Invalid item ID format: %s", itemIDStr)
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}
	utils.LogDebug("Processing cancellation for item ID: %d in order ID: %d", itemID, orderID)

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Missing cancellation reason for item ID: %d, order ID: %d: %v", itemID, orderID, err)
		utils.BadRequest(c, "Reason is required for item cancellation", nil)
		return
	}
	utils.LogDebug("Cancellation reason received for item ID: %d", itemID)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", orderID, tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}
	utils.LogDebug("Started transaction for item cancellation - Order ID: %d, Item ID: %d", orderID, itemID)

	// Get order and item with necessary preloads
	var order models.Order
	if err := tx.Preload("OrderItems").First(&order, orderID).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order ID: %d with %d items", orderID, len(order.OrderItems))

	if order.UserID != user.ID {
		utils.LogError("Unauthorized access attempt - Order ID: %d, User ID: %d", orderID, user.ID)
		tx.Rollback()
		utils.Unauthorized(c, "You are not authorized to cancel items in this order")
		return
	}

	// Check order status and cancellation window
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusPaid {
		utils.LogError("Order cannot be cancelled - Order ID: %d, Status: %s", orderID, order.Status)
		tx.Rollback()
		utils.BadRequest(c, "Items can only be cancelled before shipping", nil)
		return
	}

	// Strict 30-minute cancellation window check
	timeSinceOrder := time.Since(order.CreatedAt)
	if timeSinceOrder > 30*time.Minute {
		utils.LogError("Cancellation window expired - Order ID: %d, Created: %v", orderID, order.CreatedAt)
		tx.Rollback()
		utils.BadRequest(c, fmt.Sprintf("Cancellation window (30 minutes) has expired. Time elapsed: %.0f minutes", timeSinceOrder.Minutes()), nil)
		return
	}
	utils.LogDebug("Order within cancellation window - Order ID: %d", orderID)

	// Find the specific item
	var item models.OrderItem
	found := false
	for _, i := range order.OrderItems {
		if i.ID == itemID {
			item = i
			found = true
			break
		}
	}

	if !found {
		utils.LogError("Order item not found - Order ID: %d, Item ID: %d", orderID, itemID)
		tx.Rollback()
		utils.NotFound(c, "Order item not found")
		return
	}
	utils.LogDebug("Found item ID: %d in order ID: %d", itemID, orderID)

	// Check if item is already cancelled or has a pending cancellation request
	if item.CancellationRequested {
		utils.LogError("Item already has a cancellation request - Order ID: %d, Item ID: %d, Status: %s", orderID, itemID, item.CancellationStatus)
		tx.Rollback()
		utils.BadRequest(c, "This item has already been cancelled or has a pending cancellation request", nil)
		return
	}

	// Check if item is already cancelled
	if item.CancellationStatus == "Cancelled" {
		utils.LogError("Item already cancelled - Order ID: %d, Item ID: %d", orderID, itemID)
		tx.Rollback()
		utils.BadRequest(c, "This item has already been cancelled", nil)
		return
	}

	// Calculate refund amount for this item
	// Use the final price the customer actually paid for this item
	refundAmount := item.Total - item.CouponDiscount // This is the final price after all discounts
	utils.LogInfo("Calculated refund amount: %.2f for order ID: %d, book ID: %d (final price paid: %.2f - %.2f coupon)", refundAmount, order.ID, item.BookID, item.Total, item.CouponDiscount)

	// Update item status
	item.CancellationRequested = true
	item.CancellationReason = req.Reason
	item.CancellationStatus = "Cancelled"
	item.RefundStatus = "pending"
	item.RefundAmount = refundAmount

	if err := tx.Save(&item).Error; err != nil {
		utils.LogError("Failed to update order item - Item ID: %d: %v", itemID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}
	utils.LogDebug("Updated item status to cancelled - Item ID: %d", itemID)

	// Update book stock
	if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
		utils.LogError("Failed to update book stock for book ID: %d: %v", item.BookID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update book stock", err.Error())
		return
	}
	utils.LogInfo("Updated book stock for book ID: %d, added: %d", item.BookID, item.Quantity)

	// Prepare response based on payment method
	itemResponse := gin.H{
		"id":                  item.ID,
		"cancellation_status": "Cancelled",
		"cancellation_reason": req.Reason,
		"refund_amount":       fmt.Sprintf("%.2f", refundAmount),
		"refund_details": gin.H{
			"item_total":       fmt.Sprintf("%.2f", item.Price*float64(item.Quantity)),
			"item_discount":    fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount":  fmt.Sprintf("%.2f", item.CouponDiscount),
			"final_price_paid": fmt.Sprintf("%.2f", item.Total-item.CouponDiscount),
			"total_refunded":   fmt.Sprintf("%.2f", refundAmount),
			"refund_status":    "refunded to wallet",
			"refunded_to":      "wallet",
		},
	}

	// Handle refund based on payment method
	if order.PaymentMethod == "COD" || order.PaymentMethod == "cod" {
		utils.LogDebug("No refund applicable for COD order - Order ID: %d, Item ID: %d", orderID, itemID)
		itemResponse["refund_status"] = "No refund applicable for COD orders"
	} else {
		utils.LogDebug("Processing refund for non-COD order - Order ID: %d, Item ID: %d", orderID, itemID)
		wallet, err := utils.GetOrCreateWallet(user.ID)
		if err != nil {
			utils.LogError("Failed to get wallet for user ID: %d, item ID: %d: %v", user.ID, itemID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to process refund", nil)
			return
		}

		// Create refund transaction
		reference := fmt.Sprintf("REFUND-ORDER-%d-ITEM-%d", orderID, itemID)
		description := fmt.Sprintf("Refund for cancelled item in order #%d", orderID)

		transaction, err := utils.CreateWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &order.ID, reference)
		if err != nil {
			utils.LogError("Failed to create refund transaction - Item ID: %d: %v", itemID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to create refund transaction", nil)
			return
		}
		utils.LogDebug("Created wallet transaction - Transaction ID: %d, Amount: %.2f", transaction.ID, transaction.Amount)

		// Update wallet balance
		if err := utils.UpdateWalletBalance(wallet.ID, refundAmount, models.TransactionTypeCredit); err != nil {
			utils.LogError("Failed to update wallet balance - Wallet ID: %d, Item ID: %d: %v", wallet.ID, itemID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update wallet balance", nil)
			return
		}
		utils.LogDebug("Updated wallet balance - Wallet ID: %d, Amount: %.2f", wallet.ID, refundAmount)

		// Update refund status in order item
		item.RefundStatus = "completed"
		item.RefundAmount = refundAmount
		now := time.Now()
		item.RefundedAt = &now
		if err := tx.Save(&item).Error; err != nil {
			utils.LogError("Failed to update refund status - Item ID: %d: %v", itemID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update refund status", nil)
			return
		}
		utils.LogDebug("Updated item refund status to completed - Item ID: %d", itemID)

		itemResponse["refund_details"] = gin.H{
			"item_total":       fmt.Sprintf("%.2f", item.Price*float64(item.Quantity)),
			"item_discount":    fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount":  fmt.Sprintf("%.2f", item.CouponDiscount),
			"final_price_paid": fmt.Sprintf("%.2f", item.Total-item.CouponDiscount),
			"total_refunded":   fmt.Sprintf("%.2f", refundAmount),
			"refund_status":    "refunded to wallet",
			"refunded_to":      "wallet",
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
		}
		itemResponse["wallet"] = gin.H{
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		}
	}

	// Update order totals
	order.TotalAmount -= item.Price * float64(item.Quantity)
	order.Discount -= item.Discount

	// Recalculate coupon discount proportionally
	if order.CouponDiscount > 0 {
		// Calculate the proportion of the cancelled item's total to the original order total
		originalOrderTotal := order.TotalAmount + (item.Price * float64(item.Quantity))
		cancelledItemTotal := item.Price * float64(item.Quantity)
		couponDiscountToRemove := (cancelledItemTotal / originalOrderTotal) * order.CouponDiscount
		order.CouponDiscount -= couponDiscountToRemove
	}

	// Calculate final total after all adjustments
	order.FinalTotal = order.TotalAmount - order.Discount - order.CouponDiscount
	// Add delivery charge to final total
	order.TotalWithDelivery = order.FinalTotal + order.DeliveryCharge

	if err := tx.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order totals - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}
	utils.LogDebug("Updated order totals - Order ID: %d, New Total: %.2f", orderID, order.FinalTotal)

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		utils.InternalServerError(c, "Failed to process cancellation", nil)
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d, item ID: %d", orderID, itemID)

	utils.Success(c, "Item cancelled successfully", gin.H{
		"item": itemResponse,
		"order": gin.H{
			"id":              order.ID,
			"total_amount":    fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":        fmt.Sprintf("%.2f", order.Discount),
			"coupon_discount": fmt.Sprintf("%.2f", order.CouponDiscount),
			"coupon_code":     order.CouponCode,
			"final_total":     fmt.Sprintf("%.2f", order.TotalWithDelivery),
		},
	})
}
