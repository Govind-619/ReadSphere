package controllers

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/utils"

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
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)

	// Get pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")

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
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if date := c.Query("date"); date != "" {
		query = query.Where("DATE(created_at) = ?", date)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		utils.InternalServerError(c, "Failed to count orders", nil)
		return
	}

	// Apply sorting
	switch sortBy {
	case "id", "status", "final_total", "created_at":
		query = query.Order(fmt.Sprintf("%s %s", sortBy, order))
	default:
		query = query.Order(fmt.Sprintf("created_at %s", order))
	}

	// Apply pagination
	offset := (page - 1) * limit
	var orders []models.Order
	if err := query.Offset(offset).Limit(limit).Preload("OrderItems.Book").Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch orders", nil)
		return
	}

	// Prepare order summaries
	summaries := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		summaries = append(summaries, gin.H{
			"id":           o.ID,
			"date":         o.CreatedAt.Format("2006-01-02 15:04:05"),
			"status":       o.Status,
			"final_total":  fmt.Sprintf("%.2f", o.FinalTotal),
			"item_count":   len(o.OrderItems),
			"payment_mode": o.PaymentMethod,
		})
	}

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
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book").Preload("Address").Preload("User").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.NotFound(c, "Order not found")
		return
	}

	// Prepare minimal items with IDs
	items := make([]gin.H, 0, len(order.OrderItems))
	for _, item := range order.OrderItems {
		// Use CalculateOfferDetails for current book/price
		finalUnitPrice, _, discountAmount, _ := utils.CalculateOfferDetails(item.Price, item.Book.ID, item.Book.CategoryID)
		items = append(items, gin.H{
			"id":          item.ID,
			"book_id":     item.BookID,
			"name":        item.Book.Name,
			"quantity":    item.Quantity,
			"price":       fmt.Sprintf("%.2f", item.Price),
			"discount":    fmt.Sprintf("%.2f", discountAmount*float64(item.Quantity)),
			"final_price": fmt.Sprintf("%.2f", finalUnitPrice),
			"total":       fmt.Sprintf("%.2f", item.Total),
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

	resp := gin.H{
		"order_id":       order.ID,
		"date":           order.CreatedAt.Format("2006-01-02 15:04:05"),
		"status":         order.Status,
		"payment_mode":   order.PaymentMethod,
		"address":        address,
		"items":          items,
		"initial_amount": fmt.Sprintf("%.2f", order.TotalAmount),
		"discount":       fmt.Sprintf("%.2f", order.Discount),
		"final_total":    fmt.Sprintf("%.2f", order.FinalTotal),
		"actions": gin.H{
			"can_cancel": time.Since(order.CreatedAt) <= 30*time.Minute &&
				(order.Status == models.OrderStatusPlaced || order.Status == models.OrderStatusProcessing),
			"can_return": order.Status == models.OrderStatusDelivered,
		},
	}

	utils.Success(c, "Order details retrieved successfully", resp)
}

// CancelOrder cancels an entire order
func CancelOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)

	// Parse order ID
	orderIDStr := c.Param("id")
	orderID, err := strconv.ParseUint(orderIDStr, 10, 32)
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	// Parse cancellation reason
	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Reason is required", nil)
		return
	}

	// Get the order with all items
	var order models.Order
	if err := config.DB.Preload("OrderItems").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.NotFound(c, "Order not found")
		return
	}

	// Check if order is already cancelled
	if order.Status == models.OrderStatusCancelled {
		utils.BadRequest(c, "Order already cancelled", nil)
		return
	}

	// Check if order can be cancelled based on status and time
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusProcessing {
		utils.BadRequest(c, "Order cannot be cancelled at this stage", nil)
		return
	}

	// Check 30-minute cancellation window
	if time.Since(order.CreatedAt) > 30*time.Minute {
		utils.BadRequest(c, "Cancellation window (30 minutes) has expired", nil)
		return
	}

	// Start a database transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	// Restore stock for each book
	for _, item := range order.OrderItems {
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to restore book stock"})
			return
		}
	}

	// Update order status and details
	order.Status = models.OrderStatusCancelled
	order.CancellationReason = req.Reason
	order.RefundStatus = "pending"
	order.RefundAmount = order.FinalTotal
	order.RefundedToWallet = true
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order"})
		return
	}

	// Only process refund if payment was not COD
	var walletRefundProcessed bool
	var wallet *models.Wallet
	var transaction *models.WalletTransaction

	if order.PaymentMethod != "COD" && order.PaymentMethod != "cod" {
		// Get or create wallet
		wallet, err = utils.GetOrCreateWallet(user.ID)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
			return
		}

		// Create a wallet transaction
		orderIDUint := uint(orderID)
		reference := fmt.Sprintf("REFUND-ORDER-%d", orderID)
		description := fmt.Sprintf("Refund for cancelled order #%d", orderID)

		transaction, err = utils.CreateWalletTransaction(wallet.ID, order.FinalTotal, models.TransactionTypeCredit, description, &orderIDUint, reference)
		if err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create transaction"})
			return
		}

		// Update wallet balance
		if err := utils.UpdateWalletBalance(wallet.ID, order.FinalTotal, models.TransactionTypeCredit); err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update wallet balance"})
			return
		}

		// Update order refund status
		now := time.Now()
		order.RefundStatus = "completed"
		order.RefundedAt = &now

		if err := tx.Save(&order).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update order refund status"})
			return
		}

		walletRefundProcessed = true
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Prepare response based on whether wallet refund was processed
	if order.PaymentMethod == "COD" || order.PaymentMethod == "cod" {
		c.JSON(http.StatusOK, gin.H{
			"message": "Order cancelled",
			"order": gin.H{
				"id":            order.ID,
				"status":        order.Status,
				"refund_status": "No refund applicable for COD orders",
			},
		})
	} else if walletRefundProcessed {
		c.JSON(http.StatusOK, gin.H{
			"message": "Order cancelled and refunded to wallet",
			"order": gin.H{
				"id":            order.ID,
				"status":        order.Status,
				"refund_amount": fmt.Sprintf("%.2f", order.RefundAmount),
				"refund_status": order.RefundStatus,
				"refunded_at":   order.RefundedAt.Format("2006-01-02 15:04:05"),
			},
			"transaction": gin.H{
				"id":          transaction.ID,
				"wallet_id":   transaction.WalletID,
				"amount":      fmt.Sprintf("%.2f", transaction.Amount),
				"type":        transaction.Type,
				"description": transaction.Description,
				"order_id":    transaction.OrderID,
				"reference":   transaction.Reference,
				"status":      "success",
			},
			"wallet": gin.H{
				"balance": fmt.Sprintf("%.2f", wallet.Balance),
			},
		})
	}
}

// CancelOrderItem cancels a single item in an order within 30 minutes of ordering
func CancelOrderItem(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	itemIDStr := c.Param("item_id")
	var itemID uint
	_, err = fmt.Sscanf(itemIDStr, "%d", &itemID)
	if err != nil || itemID == 0 {
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Reason is required for item cancellation", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	// Get order and item with necessary preloads
	var order models.Order
	if err := tx.Preload("OrderItems").First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Order not found")
		return
	}

	if order.UserID != user.ID {
		tx.Rollback()
		utils.Unauthorized(c, "You are not authorized to cancel items in this order")
		return
	}

	// Check order status and cancellation window
	if order.Status != models.OrderStatusPlaced && order.Status != models.OrderStatusProcessing {
		tx.Rollback()
		utils.BadRequest(c, "Items can only be cancelled before shipping", nil)
		return
	}

	// Strict 30-minute cancellation window check
	timeSinceOrder := time.Since(order.CreatedAt)
	if timeSinceOrder > 30*time.Minute {
		tx.Rollback()
		utils.BadRequest(c, fmt.Sprintf("Cancellation window (30 minutes) has expired. Time elapsed: %.0f minutes", timeSinceOrder.Minutes()), nil)
		return
	}

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
		tx.Rollback()
		utils.NotFound(c, "Order item not found")
		return
	}

	// Process cancellation immediately
	// Restore stock
	if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock + ?", item.Quantity)).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to restore book stock", nil)
		return
	}

	// Update item status
	item.CancellationRequested = true
	item.CancellationReason = req.Reason
	item.CancellationStatus = "Cancelled"

	if err := tx.Save(&item).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}

	// Calculate refund amount for this item
	itemTotal := item.Total
	refundAmount := itemTotal

	// Prepare response based on payment method
	itemResponse := gin.H{
		"id":                  item.ID,
		"cancellation_status": "Cancelled",
		"cancellation_reason": req.Reason,
	}

	// Handle refund based on payment method
	if order.PaymentMethod == "COD" || order.PaymentMethod == "cod" {
		itemResponse["refund_status"] = "No refund applicable for COD orders"
	} else {
		wallet, err := utils.GetOrCreateWallet(user.ID)
		if err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to process refund", nil)
			return
		}

		// Create refund transaction
		reference := fmt.Sprintf("REFUND-ORDER-%d-ITEM-%d", orderID, itemID)
		description := fmt.Sprintf("Refund for cancelled item in order #%d", orderID)

		transaction, err := utils.CreateWalletTransaction(wallet.ID, refundAmount, models.TransactionTypeCredit, description, &order.ID, reference)
		if err != nil {
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

		// Update refund status in order item
		item.RefundStatus = "completed"
		item.RefundAmount = refundAmount
		item.RefundedAt = &time.Time{}
		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update refund status", nil)
			return
		}

		itemResponse["refund_amount"] = fmt.Sprintf("%.2f", refundAmount)
		itemResponse["refund_details"] = gin.H{
			"item_total":     fmt.Sprintf("%.2f", itemTotal),
			"total_refunded": fmt.Sprintf("%.2f", refundAmount),
			"refund_status":  "completed",
			"refunded_to":    "wallet",
		}
		itemResponse["transaction"] = gin.H{
			"id":          transaction.ID,
			"wallet_id":   transaction.WalletID,
			"amount":      fmt.Sprintf("%.2f", transaction.Amount),
			"type":        transaction.Type,
			"description": transaction.Description,
			"order_id":    transaction.OrderID,
			"reference":   transaction.Reference,
			"status":      "success",
		}
		itemResponse["wallet"] = gin.H{
			"balance": fmt.Sprintf("%.2f", wallet.Balance),
		}
	}

	// Update order totals
	order.TotalAmount -= item.Price * float64(item.Quantity)
	order.Discount -= item.Discount
	order.FinalTotal = order.TotalAmount - order.Discount

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to process cancellation", nil)
		return
	}

	utils.Success(c, "Item cancelled successfully", gin.H{
		"item": itemResponse,
		"order": gin.H{
			"id":           order.ID,
			"total_amount": fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":     fmt.Sprintf("%.2f", order.Discount),
			"final_total":  fmt.Sprintf("%.2f", order.FinalTotal),
		},
	})
}

// ReturnOrderItem submits a request to return a single item from a delivered order
func ReturnOrderItem(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}
	itemIDStr := c.Param("item_id")
	var itemID uint
	_, err = fmt.Sscanf(itemIDStr, "%d", &itemID)
	if err != nil || itemID == 0 {
		utils.BadRequest(c, "Invalid item ID", nil)
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Reason is required for return request", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	// Get order and item with necessary preloads
	var order models.Order
	if err := tx.Preload("OrderItems").First(&order, orderID).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Order not found")
		return
	}

	if order.UserID != user.ID {
		tx.Rollback()
		utils.Unauthorized(c, "You are not authorized to return items in this order")
		return
	}

	// Check if order is delivered
	if order.Status != models.OrderStatusDelivered {
		tx.Rollback()
		utils.BadRequest(c, "Items can only be returned after delivery", nil)
		return
	}

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
		tx.Rollback()
		utils.NotFound(c, "Order item not found")
		return
	}

	// Check if item is already returned or has a pending return
	if item.ReturnRequested {
		tx.Rollback()
		utils.BadRequest(c, "Return already requested for this item", nil)
		return
	}

	// Update item status
	item.ReturnRequested = true
	item.ReturnReason = req.Reason
	item.ReturnStatus = "Pending"

	if err := tx.Save(&item).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order item", nil)
		return
	}

	// Calculate potential refund amount
	itemTotal := item.Total
	refundAmount := itemTotal

	// Prepare response
	itemResponse := gin.H{
		"id":            item.ID,
		"return_status": "Pending",
		"return_reason": req.Reason,
		"refund_details": gin.H{
			"item_total":           fmt.Sprintf("%.2f", itemTotal),
			"total_to_be_refunded": fmt.Sprintf("%.2f", refundAmount),
			"refund_status":        "Pending admin approval",
			"refund_to":            "wallet",
		},
	}

	// Update order status
	order.HasItemReturnRequests = true
	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to process return request", nil)
		return
	}

	utils.Success(c, "Return request submitted successfully", gin.H{
		"item": itemResponse,
		"order": gin.H{
			"id":     order.ID,
			"status": order.Status,
		},
		"note": "Your return request has been submitted. Our team will review it and process accordingly.",
	})
}

// ReturnOrder allows user to request a return for all items in a delivered order
func ReturnOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user := userVal.(models.User)
	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid order ID", nil)
		return
	}

	var req struct {
		Reason string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Return reason is required", nil)
		return
	}

	// Get order with items and their categories
	var order models.Order
	if err := config.DB.Preload("OrderItems.Book.Category").Where("id = ? AND user_id = ?", orderID, user.ID).First(&order).Error; err != nil {
		utils.NotFound(c, "Order not found")
		return
	}

	if order.Status != models.OrderStatusDelivered {
		utils.BadRequest(c, "Only delivered orders can be returned", nil)
		return
	}

	// Check return window for each item
	for _, item := range order.OrderItems {
		returnWindow := 7 * 24 * time.Hour // Default 7 days
		if item.Book.Category.ReturnWindow > 0 {
			returnWindow = time.Duration(item.Book.Category.ReturnWindow) * 24 * time.Hour
		}
		if time.Since(order.UpdatedAt) > returnWindow {
			utils.BadRequest(c, fmt.Sprintf("Return window has expired for some items (max %d days)", int(returnWindow.Hours()/24)), nil)
			return
		}
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to begin transaction", nil)
		return
	}

	// Mark all items for return
	for _, item := range order.OrderItems {
		item.ReturnRequested = true
		item.ReturnReason = req.Reason
		item.ReturnStatus = "Pending"
		if err := tx.Save(&item).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update order items", nil)
			return
		}
	}

	// Update order status
	order.Status = models.OrderStatusReturnRequested
	order.ReturnReason = req.Reason
	order.HasItemReturnRequests = true
	order.UpdatedAt = time.Now()

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update order", nil)
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to submit return request", nil)
		return
	}

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
