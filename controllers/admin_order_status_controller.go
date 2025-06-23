package controllers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminUpdateOrderStatus updates the status of an order
func AdminUpdateOrderStatus(c *gin.Context) {
	utils.LogInfo("AdminUpdateOrderStatus called")

	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.LogError("Invalid admin type in context")
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	utils.LogDebug("Admin authenticated: %s", adminModel.Email)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogDebug("Processing order ID: %d", orderID)

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		utils.LogError("Invalid status in request: %v", err)
		utils.BadRequest(c, "Status is required", nil)
		return
	}
	utils.LogDebug("Requested status update to: %s", req.Status)

	validStatuses := []string{"Pending", "Shipped", "Out for Delivery", "Delivered", "Cancelled"}
	found := false
	for _, s := range validStatuses {
		if strings.EqualFold(s, req.Status) {
			found = true
			break
		}
	}
	if !found {
		utils.LogError("Invalid status requested: %s", req.Status)
		utils.BadRequest(c, "Invalid status", gin.H{
			"valid_statuses": validStatuses,
		})
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to begin transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}
	utils.LogDebug("Started database transaction")

	var order models.Order
	if err := tx.First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.LogError("Order not found: %v", err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order with current status: %s", order.Status)

	// Prevent cancellation if order is already shipped, delivered, or out for delivery
	if strings.EqualFold(req.Status, "Cancelled") &&
		(strings.EqualFold(order.Status, "Shipped") ||
			strings.EqualFold(order.Status, "Delivered") ||
			strings.EqualFold(order.Status, "Out for Delivery")) {
		tx.Rollback()
		utils.LogError("Cannot cancel order %d: already %s", orderID, order.Status)
		utils.BadRequest(c, "Cannot cancel an order that is already shipped, out for delivery, or delivered", nil)
		return
	}

	// If status is being set to Cancelled and order is not shipped, restock books
	shouldRestock := false
	if strings.EqualFold(req.Status, "Cancelled") &&
		(strings.EqualFold(order.Status, "Pending") ||
			strings.EqualFold(order.Status, "Processing")) {
		shouldRestock = true
		utils.LogDebug("Order will be restocked due to cancellation")
	}

	order.Status = req.Status
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update order status: %v", err)
		utils.InternalServerError(c, "Failed to update order status", nil)
		return
	}
	utils.LogDebug("Updated order status to: %s", order.Status)

	if shouldRestock {
		var items []models.OrderItem
		if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to fetch order items: %v", err)
			utils.InternalServerError(c, "Failed to fetch order items", nil)
			return
		}
		utils.LogDebug("Found %d items to restock", len(items))

		for _, item := range items {
			if err := tx.Model(&models.Book{}).
				Where("id = ?", item.BookID).
				UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				tx.Rollback()
				utils.LogError("Failed to update book stock for book %d: %v", item.BookID, err)
				utils.InternalServerError(c, "Failed to update book stock", nil)
				return
			}
			utils.LogDebug("Restocked book %d with %d units", item.BookID, item.Quantity)
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to save changes", nil)
		return
	}
	utils.LogDebug("Successfully committed transaction")

	// Reload full order with all required relations for response
	var fullOrder models.Order
	if err := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		First(&fullOrder, orderID).Error; err != nil {
		utils.LogError("Failed to reload order details: %v", err)
		utils.InternalServerError(c, "Failed to reload order details", nil)
		return
	}
	utils.LogDebug("Reloaded order with all relations")

	// Prepare items response
	var items []gin.H
	for _, item := range fullOrder.OrderItems {
		items = append(items, gin.H{
			"id":       item.ID,
			"name":     item.Book.Name,
			"category": item.Book.Category.Name,
			"genre":    item.Book.Genre.Name,
			"quantity": item.Quantity,
			"price":    fmt.Sprintf("%.2f", item.Price),
			"total":    fmt.Sprintf("%.2f", item.Total),
		})
	}
	utils.LogDebug("Prepared response for %d order items", len(items))

	utils.LogInfo("Successfully updated order %d status to %s", orderID, order.Status)
	utils.Success(c, "Order status updated successfully", gin.H{
		"order": gin.H{
			"id":                  fullOrder.ID,
			"username":            fullOrder.User.Username,
			"email":               fullOrder.User.Email,
			"status":              fullOrder.Status,
			"total_amount":        fmt.Sprintf("%.2f", fullOrder.TotalAmount),
			"discount":            fmt.Sprintf("%.2f", fullOrder.Discount),
			"coupon_discount":     fmt.Sprintf("%.2f", fullOrder.CouponDiscount),
			"coupon_code":         fullOrder.CouponCode,
			"delivery_charge":     fmt.Sprintf("%.2f", fullOrder.DeliveryCharge),
			"total_with_delivery": fmt.Sprintf("%.2f", fullOrder.TotalWithDelivery),
			"final_total":         fmt.Sprintf("%.2f", fullOrder.FinalTotal),
			"created_at":          fullOrder.CreatedAt.Format("2006-01-02 15:04:05"),
			"updated_at":          fullOrder.UpdatedAt.Format("2006-01-02 15:04:05"),
			"payment_mode":        fullOrder.PaymentMethod,
			"items":               items,
		},
	})
}
