package controllers

import (
	"fmt"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AdminListOrders lists all orders with search, filter, sort, and pagination
func AdminListOrders(c *gin.Context) {
	utils.LogInfo("AdminListOrders called")

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

	query := config.DB.Preload("User").Preload("OrderItems").Preload("Address")

	// Filtering
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
		utils.LogDebug("Applied status filter: %s", status)
	}
	if user := c.Query("user"); user != "" {
		query = query.Joins("JOIN users ON users.id = orders.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ?", "%"+user+"%", "%"+user+"%")
		utils.LogDebug("Applied user filter: %s", user)
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(orders.created_at) = ?", date)
		utils.LogDebug("Applied date filter: %s", date)
	}
	if id := c.Query("id"); id != "" {
		query = query.Where("orders.id = ?", id)
		utils.LogDebug("Applied ID filter: %s", id)
	}

	// General search
	if search := c.Query("search"); search != "" {
		searchLike := "%" + search + "%"
		query = query.Joins("JOIN users ON users.id = orders.user_id").
			Where("CAST(orders.id AS TEXT) ILIKE ? OR users.username ILIKE ? OR users.email ILIKE ?",
				searchLike, searchLike, searchLike)
		utils.LogDebug("Applied search filter: %s", search)
	}

	// Sorting
	sortField := c.DefaultQuery("sort", "created_at")
	orderDir := c.DefaultQuery("order", "desc")
	if orderDir != "asc" && orderDir != "desc" {
		orderDir = "desc"
	}
	query = query.Order(sortField + " " + orderDir)
	utils.LogDebug("Applied sorting: %s %s", sortField, orderDir)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	utils.LogDebug("Applied pagination - Page: %d, Limit: %d", page, limit)

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.LogError("Failed to count orders: %v", err)
		utils.InternalServerError(c, "Failed to count orders", err.Error())
		return
	}
	utils.LogDebug("Total orders found: %d", total)

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders: %v", err)
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders", len(orders))

	// Prepare minimal order response
	var orderResponses []gin.H
	for _, order := range orders {
		orderResponses = append(orderResponses, gin.H{
			"id":           order.ID,
			"username":     order.User.Username,
			"email":        order.User.Email,
			"status":       order.Status,
			"final_total":  fmt.Sprintf("%.2f", order.FinalTotal),
			"created_at":   order.CreatedAt.Format("2006-01-02 15:04:05"),
			"item_count":   len(order.OrderItems),
			"payment_mode": order.PaymentMethod,
		})
	}
	utils.LogDebug("Prepared response for %d orders", len(orderResponses))

	utils.LogInfo("Successfully retrieved %d orders", len(orderResponses))
	utils.Success(c, "Orders retrieved successfully", gin.H{
		"orders": orderResponses,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  (total + int64(limit) - 1) / int64(limit),
		},
		"filters": gin.H{
			"status": c.Query("status"),
			"date":   c.Query("date"),
			"id":     c.Query("id"),
			"user":   c.Query("user"),
			"search": c.Query("search"),
		},
		"sort": gin.H{
			"by":    sortField,
			"order": orderDir,
		},
	})
}

// AdminGetOrderDetails returns order details for admin
func AdminGetOrderDetails(c *gin.Context) {
	utils.LogInfo("AdminGetOrderDetails called")

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
	utils.LogDebug("Fetching details for order ID: %d", orderID)

	var order models.Order
	if err := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		First(&order, orderID).Error; err != nil {
		utils.LogError("Order not found: %v", err)
		utils.NotFound(c, "Order not found")
		return
	}
	utils.LogDebug("Found order for user: %s", order.User.Username)

	// Prepare items response
	var items []gin.H
	for _, item := range order.OrderItems {
		items = append(items, gin.H{
			"id":       item.ID,
			"name":     item.Book.Name,
			"category": item.Book.Category.Name,
			"genre":    item.Book.Genre.Name,
			"quantity": item.Quantity,
			"price":    fmt.Sprintf("%.2f", item.Price),
			"total":    fmt.Sprintf("%.2f", item.Total),
			"status": gin.H{
				"return_requested":    item.ReturnRequested,
				"return_status":       item.ReturnStatus,
				"return_reason":       item.ReturnReason,
				"refund_status":       item.RefundStatus,
				"refund_amount":       fmt.Sprintf("%.2f", item.RefundAmount),
				"cancellation_status": item.CancellationStatus,
				"cancellation_reason": item.CancellationReason,
			},
		})
	}
	utils.LogDebug("Prepared response for %d order items", len(items))

	utils.LogInfo("Successfully retrieved details for order ID: %d", orderID)
	utils.Success(c, "Order details retrieved successfully", gin.H{
		"order": gin.H{
			"id":           order.ID,
			"username":     order.User.Username,
			"email":        order.User.Email,
			"status":       order.Status,
			"final_total":  fmt.Sprintf("%.2f", order.FinalTotal),
			"created_at":   order.CreatedAt.Format("2006-01-02 15:04:05"),
			"payment_mode": order.PaymentMethod,
			"address": gin.H{
				"line1":       order.Address.Line1,
				"line2":       order.Address.Line2,
				"city":        order.Address.City,
				"state":       order.Address.State,
				"postal_code": order.Address.PostalCode,
			},
			"items": items,
		},
	})
}
