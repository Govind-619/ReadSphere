package controllers

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ListOrders lists all orders for the logged-in user, with optional search by ID/date/status
func ListOrders(c *gin.Context) {
	utils.LogInfo("ListOrders called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	utils.LogInfo("Processing orders list for user ID: %d", user.ID)

	// Get pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")
	utils.LogDebug("Pagination parameters - Page: %d, Limit: %d, Sort: %s, Order: %s", page, limit, sortBy, order)

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	// Build base query
	query := config.DB.Model(&models.Order{}).Where("user_id = ?", user.ID)

	// Optional filters
	if id := c.Query("id"); id != "" {
		query = query.Where("id = ?", id)
		utils.LogDebug("Filtering by order ID: %s", id)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
		utils.LogDebug("Filtering by status: %s", status)
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(created_at) = ?", date)
		utils.LogDebug("Filtering by date: %s", date)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		utils.LogError("Failed to count orders for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to count orders", nil)
		return
	}
	utils.LogDebug("Total orders found: %d", total)

	// Apply sorting
	switch sortBy {
	case "id", "status", "created_at":
		query = query.Order(fmt.Sprintf("%s %s", sortBy, order))
	case "final_total":
		query = query.Order(fmt.Sprintf("total_with_delivery %s", order))
	default:
		query = query.Order(fmt.Sprintf("created_at %s", order))
	}

	// Apply pagination
	offset := (page - 1) * limit
	var orders []models.Order
	if err := query.Offset(offset).Limit(limit).Preload("OrderItems.Book").Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to fetch orders", nil)
		return
	}
	utils.LogDebug("Retrieved %d orders for user ID: %d", len(orders), user.ID)

	// Prepare order summaries
	summaries := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		summaries = append(summaries, gin.H{
			"id":           o.ID,
			"date":         o.CreatedAt.Format("2006-01-02 15:04:05"),
			"status":       o.Status,
			"final_total":  fmt.Sprintf("%.2f", o.TotalWithDelivery),
			"item_count":   len(o.OrderItems),
			"payment_mode": o.PaymentMethod,
		})
	}

	utils.LogInfo("Successfully retrieved orders for user ID: %d", user.ID)
	utils.Success(c, "Orders retrieved successfully", gin.H{
		"orders": summaries,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  int(math.Ceil(float64(total) / float64(limit))),
		},
		"filters": gin.H{
			"status": c.Query("status"),
			"date":   c.Query("date"),
			"id":     c.Query("id"),
		},
		"sort": gin.H{
			"by":    sortBy,
			"order": order,
		},
	})
}

// GetOrderDetails returns detailed info for a specific order
func GetOrderDetails(c *gin.Context) {
	utils.LogInfo("GetOrderDetails called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.LogError("Invalid order ID format: %v", err)
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	utils.LogInfo("Processing order details for order ID: %d, user ID: %d", orderID, user.ID)

	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Preload("User").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.LogError("Order not found - Order ID: %d, User ID: %d: %v", orderID, user.ID, err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order ID: %d with %d items", orderID, len(order.OrderItems))

	// Prepare minimal items with IDs
	items := make([]gin.H, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		items = append(items, gin.H{
			"id":          item.ID,
			"book_id":     item.BookID,
			"name":        item.Book.Name,
			"quantity":    item.Quantity,
			"price":       fmt.Sprintf("%.2f", item.Price),
			"discount":    fmt.Sprintf("%.2f", item.Discount),
			"final_price": fmt.Sprintf("%.2f", (item.Total-item.CouponDiscount)/float64(item.Quantity)),
			"total":       fmt.Sprintf("%.2f", item.Total-item.CouponDiscount),
			"status": gin.H{
				"cancellation_requested": item.CancellationRequested,
				"cancellation_status":    item.CancellationStatus,
				"return_requested":       item.ReturnRequested,
				"return_status":          item.ReturnStatus,
			},
		})
	}

	// Prepare simplified address
	address := gin.H{
		"line1":       order.Address.Line1,
		"line2":       order.Address.Line2,
		"city":        order.Address.City,
		"state":       order.Address.State,
		"postal_code": order.Address.PostalCode,
	}

	// Calculate action availability with better logic
	timeSinceOrder := time.Since(order.CreatedAt)

	// For cancel action: check if order is within 30 minutes AND has appropriate status
	// Handle cases where order creation time might be in the future or there are timezone issues
	canCancel := false

	// Check if order creation time is in the future (timezone/system clock issue)
	if timeSinceOrder < 0 {
		utils.LogError("Order ID: %d has future creation time: %s (time since order: %v)",
			order.ID, order.CreatedAt.Format("2006-01-02 15:04:05"), timeSinceOrder)
	} else if timeSinceOrder > 30*time.Minute {
		// Cancellation window expired
	} else if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusPaid {
		// Order status doesn't allow cancellation
	} else {
		canCancel = true
	}

	canReturn := order.Status == models.OrderStatusDelivered

	resp := gin.H{
		"order_id":        order.ID,
		"date":            order.CreatedAt.Format("2006-01-02 15:04:05"),
		"status":          order.Status,
		"payment_mode":    order.PaymentMethod,
		"address":         address,
		"items":           items,
		"initial_amount":  fmt.Sprintf("%.2f", order.TotalAmount),
		"discount":        fmt.Sprintf("%.2f", order.Discount),
		"coupon_discount": fmt.Sprintf("%.2f", order.CouponDiscount),
		"coupon_code":     order.CouponCode,
		"subtotal":        fmt.Sprintf("%.2f", order.FinalTotal),
		"delivery_charge": fmt.Sprintf("%.2f", order.DeliveryCharge),
		"final_total":     fmt.Sprintf("%.2f", order.TotalWithDelivery),
		"actions": gin.H{
			"can_cancel": canCancel,
			"can_return": canReturn,
		},
	}

	utils.LogInfo("Successfully retrieved order details for order ID: %d", orderID)
	utils.Success(c, "Order details retrieved successfully", resp)
}
