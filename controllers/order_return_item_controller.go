package controllers

import (
	"fmt"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
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
	// Use the final price the customer actually paid for this item
	refundAmount := item.Total - item.CouponDiscount // This is the final price after all discounts
	utils.LogInfo("Calculated refund amount: %.2f for order ID: %d, book ID: %d (using existing item data)", refundAmount, order.ID, item.BookID)

	// Update item status
	item.ReturnRequested = true
	item.ReturnReason = req.Reason
	item.ReturnStatus = "Pending"
	item.RefundStatus = "pending"
	item.RefundAmount = refundAmount

	if err := tx.Save(&item).Error; err != nil {
		utils.LogError("Failed to update order item - Item ID: %d: %v", itemID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}
	utils.LogDebug("Updated item status to pending - Item ID: %d", itemID)

	// Update order to indicate it has return requests
	order.HasItemReturnRequests = true
	if err := tx.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order return requests flag - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}
	utils.LogDebug("Updated order return requests flag - Order ID: %d", orderID)

	// Note: Stock will be restored when admin approves the return request
	// Do not restore stock immediately as return requires admin approval

	// Prepare response - returns should show pending status since they require admin approval
	itemResponse := gin.H{
		"id":            item.ID,
		"return_status": "Pending",
		"return_reason": req.Reason,
		"refund_amount": fmt.Sprintf("%.2f", refundAmount),
		"refund_details": gin.H{
			"item_total":       fmt.Sprintf("%.2f", item.Price*float64(item.Quantity)),
			"item_discount":    fmt.Sprintf("%.2f", item.Discount),
			"coupon_discount":  fmt.Sprintf("%.2f", item.CouponDiscount),
			"final_price_paid": fmt.Sprintf("%.2f", item.Total-item.CouponDiscount),
			"total_refunded":   fmt.Sprintf("%.2f", refundAmount),
			"refund_status":    "pending",
			"refunded_to":      "wallet",
		},
	}

	// Calculate projected order totals after return (for response display only)
	// Use existing order data and simply subtract the refund amount
	projectedFinalTotal := order.FinalTotal - refundAmount

	// Note: Refunds for returns are processed by admin approval, not immediately
	// The refund will be processed when admin approves the return request

	// Note: Order totals will be updated when admin approves the return request
	// Do not update order totals immediately as return requires admin approval

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		utils.InternalServerError(c, "Failed to process return", nil)
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d, item ID: %d", orderID, itemID)

	utils.Success(c, "Return request submitted successfully", gin.H{
		"item": itemResponse,
		"order": gin.H{
			"id":                  order.ID,
			"total_amount":        fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":            fmt.Sprintf("%.2f", order.Discount),
			"coupon_discount":     fmt.Sprintf("%.2f", order.CouponDiscount),
			"coupon_code":         order.CouponCode,
			"delivery_charge":     fmt.Sprintf("%.2f", order.DeliveryCharge),
			"total_with_delivery": fmt.Sprintf("%.2f", projectedFinalTotal+order.DeliveryCharge),
			"final_total":         fmt.Sprintf("%.2f", projectedFinalTotal),
		},
		"note": "Your return request has been submitted. Our team will review it and process accordingly. The order totals shown above reflect the projected amounts after return processing.",
	})
}
