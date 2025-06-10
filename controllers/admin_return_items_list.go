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

	// Query all orders with return requests
	var orders []models.Order
	query := config.DB.Preload("User").
		Preload("OrderItems.Book").
		Preload("OrderItems", func(db *gorm.DB) *gorm.DB {
			return db.Where("return_requested = ?", true)
		}).
		Where("has_item_return_requests = ? OR EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND return_requested = true)", true)
	utils.LogDebug("Built base query for return items")

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

	// First, get total count of return items
	var total int64
	config.DB.Model(&models.OrderItem{}).
		Where("return_requested = ?", true).
		Count(&total)
	utils.LogDebug("Total return items found: %d", total)

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
		for _, item := range order.OrderItems {
			if item.ReturnRequested {
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

				// Set default refund status
				req["refund_status"] = "pending"
				req["refund_amount"] = 0.0

				// Handle different return statuses
				switch item.ReturnStatus {
				case "Approved":
					if item.RefundedAt != nil {
						req["processed_at"] = item.RefundedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "completed"
						req["refund_amount"] = item.RefundAmount
					} else {
						req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
						req["refund_status"] = "processing"
					}
				case "Rejected":
					req["processed_at"] = order.UpdatedAt.Format("2006-01-02 15:04:05")
					req["reject_reason"] = item.ReturnReason
					req["refund_status"] = "rejected"
				case "Pending":
					req["refund_status"] = "pending"
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
