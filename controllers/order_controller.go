package controllers

import (
	"bytes"
	"fmt"
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
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Preload("User").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}
	// Prepare minimal items
	items := make([]OrderBookDetailsMinimal, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		items = append(items, OrderBookDetailsMinimal{
			ItemID:     item.ID,
			Name:       item.Book.Name,
			Price:      item.Price,
			CategoryID: item.Book.CategoryID,
			GenreID:    item.Book.GenreID,
			Quantity:   item.Quantity,
			Discount:   item.Discount,
			Total:      item.Total,
		})
	}
	// Prepare minimal user info
	name := ""
	email := ""
	if order.User.ID != 0 {
		name = order.User.FirstName + " " + order.User.LastName
		email = order.User.Email
	}
	resp := OrderDetailsMinimalResponse{
		Email:         email,
		Name:          name,
		Address:       order.Address,
		TotalAmount:   order.TotalAmount,
		Discount:      order.Discount,
		Tax:           order.Tax,
		FinalTotal:    order.FinalTotal,
		PaymentMethod: order.PaymentMethod,
		Status:        order.Status,
		Items:         items,
	}
	c.JSON(http.StatusOK, resp)
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
	c.JSON(http.StatusOK, gin.H{"message": "Order cancelled"})
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
	itemIDStr := c.Param("item_id")
	var itemID uint
	_, err = fmt.Sscanf(itemIDStr, "%d", &itemID)
	if err != nil || itemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid item ID", "debug": itemIDStr})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	var item models.OrderItem
	if err := config.DB.Where("id = ? AND order_id = ?", itemID, orderID).First(&item).Error; err != nil {
		// Debug: list all items for this order
		var items []models.OrderItem
		config.DB.Where("order_id = ?", orderID).Find(&items)
		itemIDs := make([]uint, len(items))
		for i, it := range items {
			itemIDs[i] = it.ID
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Order item not found",
			"order_id": orderID,
			"item_id": itemID,
			"item_ids_for_order": itemIDs,
		})
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
	// Reload order with all relations for response
	var fullOrder models.Order
	config.DB.Preload("OrderItems.Book").Preload("User").Preload("Address").First(&fullOrder, order.ID)
	// Prepare minimal items
	items := make([]OrderBookDetailsMinimal, 0, len(fullOrder.OrderItems))
	for _, item := range fullOrder.OrderItems {
		items = append(items, OrderBookDetailsMinimal{
			ItemID:     item.ID,
			Name:       item.Book.Name,
			Price:      item.Price,
			CategoryID: item.Book.CategoryID,
			GenreID:    item.Book.GenreID,
			Quantity:   item.Quantity,
			Discount:   item.Discount,
			Total:      item.Total,
		})
	}
	// Prepare minimal user info
	name := ""
	email := ""
	if fullOrder.User.ID != 0 {
		name = fullOrder.User.FirstName + " " + fullOrder.User.LastName
		email = fullOrder.User.Email
	}
	resp := OrderDetailsMinimalResponse{
		Email:         email,
		Name:          name,
		Address:       fullOrder.Address,
		TotalAmount:   fullOrder.TotalAmount,
		Discount:      fullOrder.Discount,
		Tax:           fullOrder.Tax,
		FinalTotal:    fullOrder.FinalTotal,
		PaymentMethod: fullOrder.PaymentMethod,
		Status:        fullOrder.Status,
		Items:         items,
	}
	c.JSON(http.StatusOK, gin.H{"message": "Order returned", "order": resp})
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
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Preload("User").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	// Optional: Add logo (uncomment if logo.png exists)
	//pdf.ImageOptions("logo.png", 150, 5, 55, 0, false, gofpdf.ImageOptions{}, 0, "")

	// Store info
	pdf.SetFont("Arial", "B", 18)
	pdf.Cell(100, 10, "Read Sphere")
	pdf.SetFont("Arial", "", 12)
	pdf.Ln(8)
	pdf.Cell(100, 8, "123 Main St, City, Country")
	pdf.Ln(8)
	pdf.Cell(100, 8, "Email: support@readsphere.com | Phone: +91-12345-67890")
	pdf.Ln(12)

	// Invoice title and order info
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(100, 10, "INVOICE")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(50, 8, "Order ID: "+strconv.Itoa(int(order.ID)))
	pdf.Cell(60, 8, "Order Date: "+order.CreatedAt.Format("2006-01-02 15:04:05"))
	pdf.Ln(8)
	pdf.Cell(50, 8, "Payment Method: "+order.PaymentMethod)
	pdf.Cell(60, 8, "Status: "+order.Status)
	pdf.Ln(8)

	// Customer and shipping info
	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(100, 8, "Billed To:")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(100, 8, order.User.FirstName+" "+order.User.LastName)
	pdf.Ln(6)
	pdf.Cell(100, 8, order.User.Email)
	pdf.Ln(6)
	pdf.Cell(100, 8, "Phone: "+order.User.Phone)
	pdf.Ln(8)

	pdf.SetFont("Arial", "B", 13)
	pdf.Cell(100, 8, "Shipping Address:")
	pdf.Ln(7)
	pdf.SetFont("Arial", "", 12)
	pdf.Cell(100, 8, order.Address.Line1)
	pdf.Ln(6)
	if order.Address.Line2 != "" {
		pdf.Cell(100, 8, order.Address.Line2)
		pdf.Ln(6)
	}
	pdf.Cell(100, 8, order.Address.City+", "+order.Address.State+", "+order.Address.Country+" - "+order.Address.PostalCode)
	pdf.Ln(10)

	// Items table header
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(70, 8, "Book", "1", 0, "C", false, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "Price", "1", 0, "C", false, 0, "")
	pdf.CellFormat(30, 8, "Total", "1", 0, "C", false, 0, "")
	pdf.Ln(-1)
	pdf.SetFont("Arial", "", 12)
	for _, item := range order.OrderItems {
		pdf.CellFormat(70, 8, item.Book.Name, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 8, strconv.Itoa(item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", item.Price), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", item.Total), "1", 0, "R", false, 0, "")
		pdf.Ln(-1)
	}

	// Summary section
	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(120, 8, "Subtotal:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", order.TotalAmount), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(120, 8, "Discount:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", order.Discount), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(120, 8, "Tax:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(30, 8, fmt.Sprintf("%.2f", order.Tax), "", 1, "R", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(120, 10, "Grand Total:", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 13)
	pdf.CellFormat(30, 10, fmt.Sprintf("%.2f", order.FinalTotal), "", 1, "R", false, 0, "")

	// Thank you note
	pdf.Ln(10)
	pdf.SetFont("Arial", "I", 12)
	pdf.Cell(0, 10, "Thank you for shopping with ReadSphere!")

	var buf bytes.Buffer
	_ = pdf.Output(&buf)
	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", "attachment; filename=invoice.pdf")
	c.Data(http.StatusOK, "application/pdf", buf.Bytes())
}
