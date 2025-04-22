package controllers

import (
	"net/http"
	"strconv"
	"time"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminListOrders lists all orders with search, filter, sort, and pagination
func AdminListOrders(c *gin.Context) {
	var orders []models.Order
	query := config.DB.Preload("OrderItems.Book").Preload("User").Preload("Address")

	// Filtering
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if user := c.Query("user"); user != "" {
		query = query.Joins("JOIN users ON users.id = orders.user_id").Where("users.name ILIKE ? OR users.email ILIKE ?", "%"+user+"%", "%"+user+"%")
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(orders.created_at) = ?", date)
	}
	if id := c.Query("id"); id != "" {
		query = query.Where("orders.id = ?", id)
	}

	// Sorting
	sort := c.DefaultQuery("sort", "created_at desc")
	query = query.Order(sort)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 20 }
	var total int64
	query.Model(&models.Order{}).Count(&total)
	query.Offset((page-1)*limit).Limit(limit).Find(&orders)

	c.JSON(http.StatusOK, gin.H{
		"orders": orders,
		"total": total,
		"page": page,
		"limit": limit,
	})
}

// AdminGetOrderDetails returns order details for admin
func AdminGetOrderDetails(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("User").Preload("Address").First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// AdminUpdateOrderStatus updates the status of an order
func AdminUpdateOrderStatus(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Status == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Status is required"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid status"})
		return
	}
	var order models.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	order.Status = req.Status
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)

	// Reload full order with relations for response
	var fullOrder models.Order
	config.DB.Preload("OrderItems.Book").Preload("User").Preload("Address").First(&fullOrder, orderID)
	c.JSON(http.StatusOK, gin.H{"message": "Order status updated", "order": fullOrder})
}

// AdminRejectReturn handles rejecting a return request for an order
func AdminRejectReturn(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Rejection reason is required"})
		return
	}
	var order models.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.Status != "Returned" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not marked as returned"})
		return
	}
	order.Status = "Return Rejected"
	order.ReturnRejectReason = req.Reason
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)

	var fullOrder models.Order
	config.DB.Preload("OrderItems.Book").Preload("User").Preload("Address").First(&fullOrder, orderID)
	c.JSON(http.StatusOK, gin.H{"message": "Return rejected", "order": fullOrder})
}

// AdminAcceptReturn handles accepting a return and refunds to user's wallet
func AdminAcceptReturn(c *gin.Context) {
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var order models.Order
	if err := config.DB.Preload("User").First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.Status != "Returned" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order is not marked as returned"})
		return
	}
	// Refund to user's wallet (assuming wallet_balance field)
	config.DB.Model(&models.User{}).Where("id = ?", order.UserID).UpdateColumn("wallet_balance", gorm.Expr("wallet_balance + ?", order.FinalTotal))
	order.Status = "Return Accepted"
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)
	c.JSON(http.StatusOK, gin.H{"message": "Return accepted and refunded", "order": order})
}
