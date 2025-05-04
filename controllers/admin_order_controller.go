package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	// General search (matches order id, user name, user email, address fields)
	if search := c.Query("search"); search != "" {
		searchLike := "%" + search + "%"
		query = query.Joins("JOIN users ON users.id = orders.user_id").Joins("JOIN addresses ON addresses.id = orders.address_id").Where(
			"CAST(orders.id AS TEXT) ILIKE ? OR users.name ILIKE ? OR users.email ILIKE ? OR addresses.street ILIKE ? OR addresses.city ILIKE ? OR addresses.state ILIKE ? OR addresses.zip_code ILIKE ?",
			searchLike, searchLike, searchLike, searchLike, searchLike, searchLike, searchLike,
		)
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
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5")) // Default limit is now 5
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 5
	}
	var total int64
	query.Model(&models.Order{}).Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Find(&orders)

	// Prepare minimal order response
	type AdminOrderMinimal struct {
		ID         uint      `json:"id"`
		Username   string    `json:"username"`
		Email      string    `json:"email"`
		Address    string    `json:"address"`
		City       string    `json:"city"`
		State      string    `json:"state"`
		FinalTotal float64   `json:"final_total"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
		ItemCount  int       `json:"item_count"`
	}
	minimalOrders := make([]AdminOrderMinimal, 0, len(orders))
	for _, o := range orders {
		minimalOrders = append(minimalOrders, AdminOrderMinimal{
			ID:         o.ID,
			Username:   o.User.Username,
			Email:      o.User.Email,
			Address:    o.Address.Line1,
			City:       o.Address.City,
			State:      o.Address.State,
			FinalTotal: o.FinalTotal,
			Status:     o.Status,
			CreatedAt:  o.CreatedAt,
			ItemCount:  len(o.OrderItems),
		})
	}

	// Format the final total for display in the response
	type DisplayOrderMinimal struct {
		ID         uint      `json:"id"`
		Username   string    `json:"username"`
		Email      string    `json:"email"`
		Address    string    `json:"address"`
		City       string    `json:"city"`
		State      string    `json:"state"`
		FinalTotal string    `json:"final_total"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
		ItemCount  int       `json:"item_count"`
	}

	displayOrders := make([]DisplayOrderMinimal, len(minimalOrders))
	for i, order := range minimalOrders {
		displayOrders[i] = DisplayOrderMinimal{
			ID:         order.ID,
			Username:   order.Username,
			Email:      order.Email,
			Address:    order.Address,
			City:       order.City,
			State:      order.State,
			FinalTotal: fmt.Sprintf("%.2f", order.FinalTotal),
			Status:     order.Status,
			CreatedAt:  order.CreatedAt,
			ItemCount:  order.ItemCount,
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"orders": displayOrders,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
		"search": gin.H{
			"term": func() string {
				if c.Query("search") != "" {
					return c.Query("search")
				}
				if c.Query("status") != "" {
					return c.Query("status")
				}
				if c.Query("user") != "" {
					return c.Query("user")
				}
				return ""
			}(),
		},
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
	if err := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	// Prepare minimal order response (same as AdminListOrders, but with all items and book/category/genre details)
	type AdminOrderDetailMinimal struct {
		ID         uint      `json:"id"`
		Username   string    `json:"username"`
		Email      string    `json:"email"`
		Address    string    `json:"address"`
		City       string    `json:"city"`
		State      string    `json:"state"`
		FinalTotal string    `json:"final_total"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
		Items      []struct {
			ID       uint   `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Genre    string `json:"genre"`
			Quantity int    `json:"quantity"`
			Price    string `json:"price"`
			Total    string `json:"total"`
		} `json:"items"`
	}
	items := make([]struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Genre    string `json:"genre"`
		Quantity int    `json:"quantity"`
		Price    string `json:"price"`
		Total    string `json:"total"`
	}, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		items = append(items, struct {
			ID       uint   `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Genre    string `json:"genre"`
			Quantity int    `json:"quantity"`
			Price    string `json:"price"`
			Total    string `json:"total"`
		}{
			ID:       item.ID,
			Name:     item.Book.Name,
			Category: item.Book.Category.Name,
			Genre:    item.Book.Genre.Name,
			Quantity: item.Quantity,
			Price:    fmt.Sprintf("%.2f", item.Price),
			Total:    fmt.Sprintf("%.2f", item.Total),
		})
	}
	resp := AdminOrderDetailMinimal{
		ID:         order.ID,
		Username:   order.User.Username,
		Email:      order.User.Email,
		Address:    order.Address.Line1,
		City:       order.Address.City,
		State:      order.Address.State,
		FinalTotal: fmt.Sprintf("%.2f", order.FinalTotal),
		Status:     order.Status,
		CreatedAt:  order.CreatedAt,
		Items:      items,
	}
	c.JSON(http.StatusOK, gin.H{"order": resp})
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
	// Prevent cancellation if order is already shipped, delivered, or out for delivery
	if strings.EqualFold(req.Status, "Cancelled") && (strings.EqualFold(order.Status, "Shipped") || strings.EqualFold(order.Status, "Delivered") || strings.EqualFold(order.Status, "Out for Delivery")) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot cancel an order that is already shipped, out for delivery, or delivered"})
		return
	}
	// If status is being set to Cancelled and order is not shipped, restock books
	shouldRestock := false
	if strings.EqualFold(req.Status, "Cancelled") && (strings.EqualFold(order.Status, "Pending") || strings.EqualFold(order.Status, "Out for Delivery")) {
		shouldRestock = true
	}
	order.Status = req.Status
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)
	if shouldRestock {
		var items []models.OrderItem
		config.DB.Where("order_id = ?", order.ID).Find(&items)
		for _, item := range items {
			config.DB.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity))
		}
	}
	// Reload full order with all required relations for response
	var fullOrder models.Order
	err = config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems.Book.Category").
		Preload("OrderItems.Book.Genre").
		Preload("OrderItems.Book").
		Preload("OrderItems").
		First(&fullOrder, orderID).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload order after update"})
		return
	}
	// Prepare minimal order response (reuse structure from AdminGetOrderDetails)
	type AdminOrderDetailMinimal struct {
		ID         uint      `json:"id"`
		Username   string    `json:"username"`
		Email      string    `json:"email"`
		Address    string    `json:"address"`
		City       string    `json:"city"`
		State      string    `json:"state"`
		FinalTotal string    `json:"final_total"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
		Items      []struct {
			ID       uint   `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Genre    string `json:"genre"`
			Quantity int    `json:"quantity"`
			Price    string `json:"price"`
			Total    string `json:"total"`
		} `json:"items"`
	}
	items := make([]struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		Category string `json:"category"`
		Genre    string `json:"genre"`
		Quantity int    `json:"quantity"`
		Price    string `json:"price"`
		Total    string `json:"total"`
	}, 0, len(fullOrder.OrderItems))
	for _, item := range fullOrder.OrderItems {
		items = append(items, struct {
			ID       uint   `json:"id"`
			Name     string `json:"name"`
			Category string `json:"category"`
			Genre    string `json:"genre"`
			Quantity int    `json:"quantity"`
			Price    string `json:"price"`
			Total    string `json:"total"`
		}{
			ID:       item.ID,
			Name:     item.Book.Name,
			Category: item.Book.Category.Name,
			Genre:    item.Book.Genre.Name,
			Quantity: item.Quantity,
			Price:    fmt.Sprintf("%.2f", item.Price),
			Total:    fmt.Sprintf("%.2f", item.Total),
		})
	}
	resp := AdminOrderDetailMinimal{
		ID:         fullOrder.ID,
		Username:   fullOrder.User.Username,
		Email:      fullOrder.User.Email,
		Address:    fullOrder.Address.Line1,
		City:       fullOrder.Address.City,
		State:      fullOrder.Address.State,
		FinalTotal: fmt.Sprintf("%.2f", fullOrder.FinalTotal),
		Status:     fullOrder.Status,
		CreatedAt:  fullOrder.CreatedAt,
		Items:      items,
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order status updated", "order": resp})
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
	var orders []models.Order
	query := config.DB.Preload("User").Preload("Address").Where("status = ? OR status = ?", "Returned", models.OrderStatusReturnRequested)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	var total int64
	query.Model(&models.Order{}).Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Find(&orders)

	type ReturnRequestMinimal struct {
		ID         uint      `json:"id"`
		Username   string    `json:"username"`
		Email      string    `json:"email"`
		Address    string    `json:"address"`
		FinalTotal float64   `json:"final_total"`
		Status     string    `json:"status"`
		CreatedAt  time.Time `json:"created_at"`
	}
	minimal := make([]ReturnRequestMinimal, 0, len(orders))
	for _, o := range orders {
		minimal = append(minimal, ReturnRequestMinimal{
			ID:         o.ID,
			Username:   o.User.Username,
			Email:      o.User.Email,
			Address:    o.Address.Line1,
			FinalTotal: o.FinalTotal,
			Status:     o.Status,
			CreatedAt:  o.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"returns": minimal,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// DEPRECATED: AdminAcceptReturn has been merged with ApproveOrderReturn in wallet_controller.go
// This function redirects to the new implementation for backwards compatibility
func AdminAcceptReturn(c *gin.Context) {
	c.JSON(http.StatusMovedPermanently, gin.H{
		"error": "This endpoint is deprecated. Please use /v1/admin/orders/:id/return/approve instead.",
	})
}
