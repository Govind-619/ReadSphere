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

// ReturnOrderItem submits a request to return a single item from a delivered order
func ReturnOrderItem(c *gin.Context) {
	utils.LogInfo("ReturnOrderItem called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing item return for user ID: %d", user.ID)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogDebug("Processing return for order ID: %d", orderID)

	itemIDStr := c.Param("item_id")
	var itemID uint
	_, err = fmt.Sscanf(itemIDStr, "%d", &itemID)
	if err != nil || itemID == 0 {
		utils.LogError("Invalid item ID format: %s", itemIDStr)
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}
	utils.LogDebug("Processing return for item ID: %d in order ID: %d", itemID, orderID)

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Missing return reason for item ID: %d, order ID: %d: %v", itemID, orderID, err)
		utils.BadRequest(c, "Reason is required for return request", nil)
		return
	}
	utils.LogDebug("Return reason received for item ID: %d", itemID)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", orderID, tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}
	utils.LogDebug("Started transaction for item return - Order ID: %d, Item ID: %d", orderID, itemID)

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
		utils.Unauthorized(c, "You are not authorized to return items in this order")
		return
	}

	// Check if order is delivered
	if order.Status != models.OrderStatusDelivered {
		utils.LogError("Order cannot be returned - Order ID: %d, Status: %s", orderID, order.Status)
		tx.Rollback()
		utils.BadRequest(c, "Items can only be returned after delivery", nil)
		return
	}
	utils.LogDebug("Order status verified for return - Order ID: %d", orderID)

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

	// Check if item is already returned or has a pending return
	if item.ReturnRequested {
		utils.LogError("Return already requested - Order ID: %d, Item ID: %d", orderID, itemID)
		tx.Rollback()
		utils.BadRequest(c, "Return already requested for this item", nil)
		return
	}

	// Calculate refund amount for this item
	refundAmount := item.Price*float64(item.Quantity) - item.Discount

	if order.CouponDiscount > 0 {
		// Calculate the proportion of the returned item's total to the original order total
		originalOrderTotal := order.TotalAmount
		returnedItemTotal := item.Price*float64(item.Quantity) - item.Discount // Use discounted total
		couponDiscountToRemove := (returnedItemTotal / originalOrderTotal) * order.CouponDiscount
		refundAmount -= couponDiscountToRemove
	}
	utils.LogInfo("Calculated refund amount: %.2f for order ID: %d, book ID: %d", refundAmount, order.ID, item.BookID)

	// Update item status
	item.ReturnRequested = true
	item.ReturnReason = req.Reason
	item.ReturnStatus = "Returned"
	item.RefundStatus = "pending"
	item.RefundAmount = refundAmount

	if err := tx.Save(&item).Error; err != nil {
		utils.LogError("Failed to update order item - Item ID: %d: %v", itemID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}
	utils.LogDebug("Updated item status to returned - Item ID: %d", itemID)

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
		"id":            item.ID,
		"return_status": "Returned",
		"return_reason": req.Reason,
		"refund_amount": fmt.Sprintf("%.2f", refundAmount),
		"refund_details": gin.H{
			"item_total":      fmt.Sprintf("%.2f", item.Price*float64(item.Quantity)),
			"item_discount":   fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount": fmt.Sprintf("%.2f", order.CouponDiscount),
			"total_refunded":  fmt.Sprintf("%.2f", refundAmount),
			"refund_status":   "pending",
			"refunded_to":     "wallet",
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
		description := fmt.Sprintf("Refund for returned item in order #%d", orderID)

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
			"item_total":      fmt.Sprintf("%.2f", item.Price*float64(item.Quantity)),
			"item_discount":   fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount": fmt.Sprintf("%.2f", order.CouponDiscount),
			"total_refunded":  fmt.Sprintf("%.2f", refundAmount),
			"refund_status":   "completed",
			"refunded_to":     "wallet",
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
		// Calculate the proportion of the returned item's total to the original order total
		originalOrderTotal := order.TotalAmount + (item.Price * float64(item.Quantity))
		returnedItemTotal := item.Price*float64(item.Quantity) - item.Discount
		couponDiscountToRemove := (returnedItemTotal / originalOrderTotal) * order.CouponDiscount
		order.CouponDiscount -= couponDiscountToRemove
	}

	// Calculate final total after all adjustments
	order.FinalTotal = order.TotalAmount - order.Discount - order.CouponDiscount

	if err := tx.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order totals - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}
	utils.LogDebug("Updated order totals - Order ID: %d, New Total: %.2f", orderID, order.FinalTotal)

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		utils.InternalServerError(c, "Failed to process return", nil)
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d, item ID: %d", orderID, itemID)

	utils.Success(c, "Item returned successfully", gin.H{
		"item": itemResponse,
		"order": gin.H{
			"id":              order.ID,
			"total_amount":    fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":        fmt.Sprintf("%.2f", order.Discount),
			"coupon_discount": fmt.Sprintf("%.2f", order.CouponDiscount),
			"coupon_code":     order.CouponCode,
			"final_total":     fmt.Sprintf("%.2f", order.FinalTotal),
		},
	})
}
