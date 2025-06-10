package controllers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ReturnOrder allows user to request a return for all items in a delivered order
func ReturnOrder(c *gin.Context) {
	utils.LogInfo("ReturnOrder called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing order return for user ID: %d", user.ID)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogDebug("Processing return for order ID: %d", orderID)

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Missing return reason for order ID: %d: %v", orderID, err)
		utils.BadRequest(c, "Return reason is required", nil)
		return
	}
	utils.LogDebug("Return reason received for order ID: %d", orderID)

	// Get order with items and their categories
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book.Category").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d: %v", orderID, err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order ID: %d with %d items", orderID, len(order.OrderItems))

	if order.Status != models.OrderStatusDelivered {
		utils.LogError("Order cannot be returned - Order ID: %d, Status: %s", orderID, order.Status)
		utils.BadRequest(c, "Only delivered orders can be returned", nil)
		return
	}
	utils.LogDebug("Order status verified for return - Order ID: %d", orderID)

	// Check return window for each item
	for _, item := range order.OrderItems {
		returnWindow := 7 * 24 * time.Hour // Default 7 days
		if item.Book.Category.ReturnWindow > 0 {
			returnWindow = time.Duration(item.Book.Category.ReturnWindow) * 24 * time.Hour
		}
		if time.Since(order.UpdatedAt) > returnWindow {
			utils.LogError("Return window expired - Order ID: %d, Item ID: %d, Window: %d days",
				orderID, item.ID, int(returnWindow.Hours()/24))
			utils.BadRequest(c, fmt.Sprintf("Return window has expired for some items (max %d days)",
				int(returnWindow.Hours()/24)), nil)
			return
		}
	}
	utils.LogDebug("Return window verified for all items in order ID: %d", orderID)

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction for order ID: %d: %v", orderID, tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}
	utils.LogDebug("Started transaction for order return - Order ID: %d", orderID)

	// Mark all items for return
	for _, item := range order.OrderItems {
		item.ReturnRequested = true
		item.ReturnReason = req.Reason
		item.ReturnStatus = "Pending"
		if err := tx.Save(&item).Error; err != nil {
			utils.LogError("Failed to update order item - Order ID: %d, Item ID: %d: %v",
				orderID, item.ID, err)
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update order items", nil)
			return
		}
		utils.LogDebug("Updated item status to return requested - Order ID: %d, Item ID: %d",
			orderID, item.ID)
	}

	// Update order status
	order.Status = models.OrderStatusReturnRequested
	order.ReturnReason = req.Reason
	order.HasItemReturnRequests = true
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		utils.LogError("Failed to update order - Order ID: %d: %v", orderID, err)
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}
	utils.LogDebug("Updated order status to return requested - Order ID: %d", orderID)

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction - Order ID: %d: %v", orderID, err)
		utils.InternalServerError(c, "Failed to submit return request", nil)
		return
	}
	utils.LogInfo("Successfully committed transaction for order ID: %d", orderID)

	// Prepare response
	orderResponse := gin.H{
		"id":            order.ID,
		"status":        order.Status,
		"return_status": "Pending",
		"items_count":   len(order.OrderItems),
		"refund_details": gin.H{
			"total_amount":         fmt.Sprintf("%.2f", order.TotalAmount),
			"total_to_be_refunded": fmt.Sprintf("%.2f", order.FinalTotal),
			"refund_status":        "Pending admin approval",
			"refund_to":            "wallet",
		},
	}

	utils.Success(c, "Return request submitted successfully", gin.H{
		"order": orderResponse,
		"note":  "Your return request has been submitted. Our team will review it and process accordingly.",
	})
}
