package controllers

import (
	"bytes"
	"net/http"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
	"gorm.io/gorm"
)

// ListOrders lists all orders for the logged-in user, with optional search by ID/date/status
func ListOrders(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	var orders []models.Order
	query := config.DB.Where("user_id = ?", user.ID)

	// Optional filters
	if id := c.Query("id"); id != "" {
		query = query.Where("id = ?", id)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(created_at) = ?", date)
	}
	query.Order("created_at DESC").Preload("OrderItems.Book").Find(&orders)

	// Summarize orders
	summaries := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		summaries = append(summaries, gin.H{
			"id":          o.ID,
			"date":        o.CreatedAt.Format("2006-01-02 15:04:05"),
			"status":      o.Status,
			"final_total": o.FinalTotal,
		})
	}
	c.JSON(http.StatusOK, gin.H{"orders": summaries})
}

// GetOrderDetails returns detailed info for a specific order
func GetOrderDetails(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"order": order})
}

// CancelOrder cancels an entire order, restores stock, and records reason
func CancelOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	var order models.Order
	if err := config.DB.Preload("OrderItems").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.Status == "Cancelled" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order already cancelled"})
		return
	}
	// Restore stock
	for _, item := range order.OrderItems {
		config.DB.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity))
	}
	order.Status = "Cancelled"
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)
	// Optionally, save cancel reason in a dedicated field or table
	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled", "order": order})
}

// CancelOrderItem cancels a single item in an order
func CancelOrderItem(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	itemID, err := strconv.Atoi(c.Param("item_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	var item models.OrderItem
	if err := config.DB.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order item not found"})
		return
	}
	// Fetch the order and check ownership
	var order models.Order
	if err := config.DB.First(&order, orderID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.UserID != user.ID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "You are not authorized to cancel items in this order"})
		return
	}
	// Restore stock
	config.DB.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity))
	config.DB.Delete(&item)
	c.JSON(http.StatusOK, gin.H{"message": "Order item cancelled"})
}

// ReturnOrder allows user to return an order if delivered, reason mandatory
func ReturnOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Reason == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Return reason is required"})
		return
	}
	var order models.Order
	if err := config.DB.Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	if order.Status != "Delivered" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only delivered orders can be returned"})
		return
	}
	order.Status = "Returned"
	order.UpdatedAt = time.Now()
	config.DB.Save(&order)
	// Optionally, save return reason
	c.JSON(http.StatusOK, gin.H{"message": "Order returned", "order": order})
}

// DownloadInvoice generates and returns a PDF invoice for the order
func DownloadInvoice(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, "Invoice")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, "Order ID: "+strconv.Itoa(int(order.ID)))
	pdf.Ln(8)
	pdf.Cell(40, 10, "Date: "+order.CreatedAt.Format("2006-01-02 15:04:05"))
	pdf.Ln(8)
	pdf.Cell(40, 10, "Status: "+order.Status)
	pdf.Ln(8)
	pdf.Cell(40, 10, "Total: "+strconv.FormatFloat(order.FinalTotal, 'f', 2, 64))
	pdf.Ln(12)
	pdf.Cell(40, 10, "Items:")
	pdf.Ln(8)
	for _, item := range order.OrderItems {
		pdf.Cell(40, 10, item.Book.Name+" x"+strconv.Itoa(item.Quantity)+" - "+strconv.FormatFloat(item.Total, 'f', 2, 64))
		pdf.Ln(7)
	}
	var buf bytes.Buffer
	_ = pdf.Output(&buf)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=invoice.pdf")
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}
