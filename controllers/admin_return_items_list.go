package controllers

import (
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AdminListReturnItems lists all item return requests including processed ones
func AdminListReturnItems(c *gin.Context) {
	utils.LogInfo("AdminListReturnItems called")

	// Check if admin is in context
	_, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		utils.Unauthorized(c, "Admin not found")
		return
	}

	// Query all orders with individual item return requests (not entire order returns)
	var orders []models.Order
	query := config.DB.Preload("User").
		Preload("OrderItems.Book").
		Preload("OrderItems", func(db *gorm.DB) *gorm.DB {
			return db.Where("return_requested = ? AND return_status IN (?, ?, ?, ?, ?, ?)", true,
				models.OrderStatusReturnRequested, models.OrderStatusReturnApproved, models.OrderStatusReturnRejected,
				"Pending", "Approved", "Rejected")
		}).
		Where("EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND return_requested = true)").
		Order("updated_at DESC") // Sort by most recent first
	utils.LogDebug("Built base query for individual item returns")

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	utils.LogDebug("Pagination parameters - Page: %d, Limit: %d", page, limit)

	// First, get total count of individual item returns (not entire order returns)
	var total int64
	config.DB.Model(&models.OrderItem{}).
		Joins("JOIN orders ON order_items.order_id = orders.id").
		Where("order_items.return_requested = ? AND order_items.return_status IN (?, ?, ?, ?, ?, ?)",
			true,
			models.OrderStatusReturnRequested, models.OrderStatusReturnApproved, models.OrderStatusReturnRejected,
			"Pending", "Approved", "Rejected").
		Count(&total)
	utils.LogDebug("Total individual item returns found: %d", total)

	// Get all orders but we'll paginate the items
	if err := query.Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch return requests: %v", err)
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders with return items", len(orders))

	// Prepare response with return details
	var requests []gin.H
	itemCount := 0
	startIdx := (page - 1) * limit

	// Process orders and collect items with pagination
	for _, order := range orders {
		utils.LogDebug("Processing order ID: %d, Status: %s, HasItemReturnRequests: %v", order.ID, order.Status, order.HasItemReturnRequests)

		// Skip orders that have status "Return Requested" (these are entire order returns)
		if order.Status == models.OrderStatusReturnRequested {
			utils.LogDebug("Skipping order ID: %d - entire order return", order.ID)
			continue
		}

		for _, item := range order.OrderItems {
			utils.LogDebug("Checking item ID: %d, ReturnRequested: %v, ReturnStatus: %s", item.ID, item.ReturnRequested, item.ReturnStatus)

			// Ensure only individual item return requests are processed
			if item.ReturnRequested && (item.ReturnStatus == models.OrderStatusReturnRequested || item.ReturnStatus == models.OrderStatusReturnApproved || item.ReturnStatus == models.OrderStatusReturnRejected ||
				item.ReturnStatus == "Pending" || item.ReturnStatus == "Approved" || item.ReturnStatus == "Rejected") {
				utils.LogDebug("Processing return item ID: %d with status: %s", item.ID, item.ReturnStatus)

				// Skip items before the start index
				if itemCount < startIdx {
					itemCount++
					continue
				}
				// Break if we've reached our limit
				if len(requests) >= limit {
					break
				}

				// Initialize base request object
				req := gin.H{
					"order_id":     order.ID,
					"username":     order.User.Username,
					"email":        order.User.Email,
					"item_id":      item.ID,
					"book_name":    item.Book.Name,
					"quantity":     item.Quantity,
					"total":        item.Total,
					"reason":       item.ReturnReason,
					"status":       item.ReturnStatus,
					"requested_at": order.UpdatedAt.Format("2006-01-02 15:04:05"),
				}

				// Calculate refund amount based on item total
				refundAmount := item.Total
				req["refund_amount"] = refundAmount

				// Handle different return statuses (support both old and new formats)
				switch item.ReturnStatus {
				case models.OrderStatusReturnApproved, "Approved":
					if item.RefundedAt != nil {
						req["processed_at"] = item.RefundedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "completed"
						req["refund_amount"] = item.RefundAmount // Use actual refunded amount if available
					} else {
						req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "processing"
						req["refund_amount"] = refundAmount
					}
				case models.OrderStatusReturnRejected, "Rejected":
					req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
					req["reject_reason"] = item.ReturnReason
					req["refund_status"] = "rejected"
					req["refund_amount"] = 0.0 // No refund for rejected items
				case models.OrderStatusReturnRequested, "Pending":
					req["refund_status"] = "pending"
					req["refund_amount"] = refundAmount
				}

				requests = append(requests, req)
				itemCount++
			}
		}
		if len(requests) >= limit {
			break
		}
	}
	utils.LogDebug("Processed %d return items for response", len(requests))

	utils.LogInfo("Successfully retrieved %d return items", len(requests))
	utils.Success(c, "Return requests retrieved successfully", gin.H{
		"requests": requests,
		"pagination": gin.H{
			"page":        page,
			"per_page":    limit,
			"total":       total,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
			"has_more":    (int64(page)*int64(limit) < total),
		},
	})
}
