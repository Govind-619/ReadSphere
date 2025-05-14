package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminListOrders lists all orders with search, filter, sort, and pagination
func AdminListOrders(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	query := config.DB.Preload("User").Preload("OrderItems").Preload("Address")

	// Filtering
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if user := c.Query("user"); user != "" {
		query = query.Joins("JOIN users ON users.id = orders.user_id").
			Where("users.username ILIKE ? OR users.email ILIKE ?", "%"+user+"%", "%"+user+"%")
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(orders.created_at) = ?", date)
	}
	if id := c.Query("id"); id != "" {
		query = query.Where("orders.id = ?", id)
	}

	// General search
	if search := c.Query("search"); search != "" {
		searchLike := "%" + search + "%"
		query = query.Joins("JOIN users ON users.id = orders.user_id").
			Where("CAST(orders.id AS TEXT) ILIKE ? OR users.username ILIKE ? OR users.email ILIKE ?",
				searchLike, searchLike, searchLike)
	}

	// Sorting
	sortField := c.DefaultQuery("sort", "created_at")
	orderDir := c.DefaultQuery("order", "desc")
	if orderDir != "asc" && orderDir != "desc" {
		orderDir = "desc"
	}
	query = query.Order(sortField + " " + orderDir)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.InternalServerError(c, "Failed to count orders", err.Error())
		return
	}

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}

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
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	var order models.Order
	if err := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		First(&order, orderID).Error; err != nil {
		utils.NotFound(c, "Order not found")
		return
	}

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

// AdminUpdateOrderStatus updates the status of an order
func AdminUpdateOrderStatus(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		utils.BadRequest(c, "Status is required", nil)
		return
	}

	validStatuses := []string{"Pending", "Shipped", "Out for Delivery", "Delivered", "Cancelled"}
	found := false
	for _, s := range validStatuses {
		if strings.EqualFold(s, req.Status) {
			found = true
			break
		}
	}
	if !found {
		utils.BadRequest(c, "Invalid status", gin.H{
			"valid_statuses": validStatuses,
		})
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	var order models.Order
	if err := tx.First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Order not found")
		return
	}

	// Prevent cancellation if order is already shipped, delivered, or out for delivery
	if strings.EqualFold(req.Status, "Cancelled") &&
		(strings.EqualFold(order.Status, "Shipped") ||
			strings.EqualFold(order.Status, "Delivered") ||
			strings.EqualFold(order.Status, "Out for Delivery")) {
		tx.Rollback()
		utils.BadRequest(c, "Cannot cancel an order that is already shipped, out for delivery, or delivered", nil)
		return
	}

	// If status is being set to Cancelled and order is not shipped, restock books
	shouldRestock := false
	if strings.EqualFold(req.Status, "Cancelled") &&
		(strings.EqualFold(order.Status, "Pending") ||
			strings.EqualFold(order.Status, "Processing")) {
		shouldRestock = true
	}

	order.Status = req.Status
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order status", nil)
		return
	}

	if shouldRestock {
		var items []models.OrderItem
		if err := tx.Where("order_id = ?", order.ID).Find(&items).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to fetch order items", nil)
			return
		}

		for _, item := range items {
			if err := tx.Model(&models.Book{}).
				Where("id = ?", item.BookID).
				UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
				tx.Rollback()
				utils.InternalServerError(c, "Failed to update book stock", nil)
				return
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to save changes", nil)
		return
	}

	// Reload full order with all required relations for response
	var fullOrder models.Order
	if err := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		First(&fullOrder, orderID).Error; err != nil {
		utils.InternalServerError(c, "Failed to reload order details", nil)
		return
	}

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

	utils.Success(c, "Order status updated successfully", gin.H{
		"order": gin.H{
			"id":           fullOrder.ID,
			"username":     fullOrder.User.Username,
			"email":        fullOrder.User.Email,
			"status":       fullOrder.Status,
			"final_total":  fmt.Sprintf("%.2f", fullOrder.FinalTotal),
			"created_at":   fullOrder.CreatedAt.Format("2006-01-02 15:04:05"),
			"updated_at":   fullOrder.UpdatedAt.Format("2006-01-02 15:04:05"),
			"payment_mode": fullOrder.PaymentMethod,
			"items":        items,
		},
	})
}

// DEPRECATED: AdminRejectReturn has been merged with RejectOrderReturn in wallet_controller.go
// This function redirects to the new implementation for backwards compatibility
func AdminRejectReturn(c *gin.Context) {
	c.JSON(http.StatusMovedPermanently, gin.H{
		"error": "This endpoint is deprecated. Please use /v1/admin/orders/:id/return/reject instead.",
	})
}

// AdminListReturnRequests lists all return requests pending admin action
func AdminListReturnRequests(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 10 // Fixed limit of 10 items per page

	// Determine if we're showing all returns or only pending ones based on the route
	path := c.Request.URL.Path
	showAllReturns := strings.HasSuffix(path, "/returns")

	// Build base query with proper ordering (pending first)
	query := config.DB.
		Preload("User").
		Preload("OrderItems.Book") // Added Book preload for item details

	// Apply different filters based on the route
	if showAllReturns {
		// For /returns route - show all returns including approved and rejected
		query = query.Where(
			"EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_requested = true)",
		)
	} else {
		// For /return-items route - show only orders with pending returns
		query = query.Where(
			"EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_requested = true AND order_items.return_status = ?)",
			models.OrderStatusReturnRequested,
		)
	}

	// Order by pending status first, then by creation date
	query = query.Order("CASE WHEN EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_status = '" + models.OrderStatusReturnRequested + "') THEN 0 ELSE 1 END, created_at DESC")

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.InternalServerError(c, "Failed to count return requests", err.Error())
		return
	}

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}

	// Prepare response with return details
	var returnRequests []gin.H
	for _, order := range orders {
		if showAllReturns {
			// Simplified response for /returns route
			var returnItems []gin.H
			for _, item := range order.OrderItems {
				if item.ReturnRequested {
					returnItems = append(returnItems, gin.H{
						"id":            item.ID,
						"name":          item.Book.Name,
						"status":        item.ReturnStatus,
						"reason":        item.ReturnReason,
						"refund_status": item.RefundStatus,
					})
				}
			}

			if len(returnItems) > 0 {
				returnRequests = append(returnRequests, gin.H{
					"id":          order.ID,
					"user":        order.User.Username,
					"date":        order.CreatedAt.Format("2006-01-02"),
					"items":       returnItems,
					"has_pending": hasPendingReturns(order.OrderItems),
				})
			}
		} else {
			// Detailed response for /return-items route
			var stats = struct {
				pending  int
				approved int
				rejected int
				total    int
			}{}

			for _, item := range order.OrderItems {
				if item.ReturnRequested {
					stats.total++
					switch item.ReturnStatus {
					case models.OrderStatusReturnRequested:
						stats.pending++
					case models.OrderStatusReturnApproved:
						stats.approved++
					case models.OrderStatusReturnRejected:
						stats.rejected++
					}
				}
			}

			returnRequests = append(returnRequests, gin.H{
				"id":            order.ID,
				"username":      order.User.Username,
				"email":         order.User.Email,
				"status":        order.Status,
				"final_total":   fmt.Sprintf("%.2f", order.FinalTotal),
				"created_at":    order.CreatedAt.Format("2006-01-02 15:04:05"),
				"return_reason": order.ReturnReason,
				"return_status": gin.H{
					"total_items":    stats.total,
					"pending_items":  stats.pending,
					"approved_items": stats.approved,
					"rejected_items": stats.rejected,
					"is_all_pending": stats.pending == stats.total && stats.total > 0,
				},
			})
		}
	}

	utils.Success(c, "Return requests retrieved successfully", gin.H{
		"returns": returnRequests,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  (total + int64(limit) - 1) / int64(limit),
			"has_more":     (int64(page)*int64(limit) < total),
		},
	})
}

// hasPendingReturns checks if any order items have pending return status
func hasPendingReturns(items []models.OrderItem) bool {
	for _, item := range items {
		if item.ReturnRequested && item.ReturnStatus == models.OrderStatusReturnRequested {
			return true
		}
	}
	return false
}

// DEPRECATED: AdminAcceptReturn has been merged with ApproveOrderReturn in wallet_controller.go
// This function redirects to the new implementation for backwards compatibility
func AdminAcceptReturn(c *gin.Context) {
	c.JSON(http.StatusMovedPermanently, gin.H{
		"error": "This endpoint is deprecated. Please use /v1/admin/orders/:id/return/approve instead.",
	})
}

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

// AdminListAllReturns lists all return requests including approved and rejected ones
func AdminListAllReturns(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 10 // Fixed limit of 10 items per page

	// Build base query with proper ordering (pending first)
	query := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems").
		Where("has_item_return_requests = ? OR EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_requested = true)", true).
		Order("CASE WHEN status = '" + models.OrderStatusReturnRequested + "' THEN 0 ELSE 1 END, created_at DESC")

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.InternalServerError(c, "Failed to count return requests", err.Error())
		return
	}

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}

	// Prepare response with return details
	var returnRequests []gin.H
	for _, order := range orders {
		// Count return items by status
		var stats = struct {
			pending  int
			approved int
			rejected int
			total    int
		}{}

		var items []gin.H
		for _, item := range order.OrderItems {
			if item.ReturnRequested {
				stats.total++
				switch item.ReturnStatus {
				case models.OrderStatusReturnRequested:
					stats.pending++
				case models.OrderStatusReturnApproved:
					stats.approved++
				case models.OrderStatusReturnRejected:
					stats.rejected++
				}

				items = append(items, gin.H{
					"id":            item.ID,
					"book_name":     item.Book.Name,
					"quantity":      item.Quantity,
					"price":         fmt.Sprintf("%.2f", item.Price),
					"total":         fmt.Sprintf("%.2f", item.Total),
					"return_status": item.ReturnStatus,
					"return_reason": item.ReturnReason,
					"refund_status": item.RefundStatus,
					"refund_amount": fmt.Sprintf("%.2f", item.RefundAmount),
				})
			}
		}

		returnRequests = append(returnRequests, gin.H{
			"id":            order.ID,
			"username":      order.User.Username,
			"email":         order.User.Email,
			"status":        order.Status,
			"final_total":   fmt.Sprintf("%.2f", order.FinalTotal),
			"created_at":    order.CreatedAt.Format("2006-01-02 15:04:05"),
			"return_reason": order.ReturnReason,
			"return_items":  items,
			"return_summary": gin.H{
				"total_items":    stats.total,
				"pending_items":  stats.pending,
				"approved_items": stats.approved,
				"rejected_items": stats.rejected,
				"is_all_pending": stats.pending == stats.total && stats.total > 0,
			},
		})
	}

	utils.Success(c, "Return requests retrieved successfully", gin.H{
		"returns": returnRequests,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  (total + int64(limit) - 1) / int64(limit),
			"has_more":     (int64(page)*int64(limit) < total),
		},
		"summary": gin.H{
			"total_requests": total,
			"showing":        len(returnRequests),
			"page":           page,
		},
	})
}
