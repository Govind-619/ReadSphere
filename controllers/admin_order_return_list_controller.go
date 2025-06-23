package controllers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AdminListReturnRequests lists all return requests pending admin action
func AdminListReturnRequests(c *gin.Context) {
	utils.LogInfo("AdminListReturnRequests called")

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

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 10 // Fixed limit of 10 items per page
	utils.LogDebug("Pagination parameters - Page: %d, Limit: %d", page, limit)

	// Determine if we're showing all returns or only pending ones based on the route
	path := c.Request.URL.Path
	showAllReturns := strings.HasSuffix(path, "/returns")
	utils.LogDebug("Route path: %s, Show all returns: %v", path, showAllReturns)

	// Build base query with proper ordering (pending first)
	query := config.DB.
		Preload("User").
		Preload("OrderItems.Book") // Added Book preload for item details

	// Apply different filters based on the route
	if showAllReturns {
		// For /returns route - show only entire order returns (orders with status "Return Requested")
		query = query.Where("status = ?", models.OrderStatusReturnRequested)
		utils.LogDebug("Applied filter for entire order returns only")
	} else {
		// For /return-items route - show only orders with pending returns
		query = query.Where(
			"EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_requested = true AND order_items.return_status = ?)",
			models.OrderStatusReturnRequested,
		)
		utils.LogDebug("Applied filter for pending returns only")
	}

	// Order by pending status first, then by creation date
	query = query.Order("CASE WHEN EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_status = '" + models.OrderStatusReturnRequested + "') THEN 0 ELSE 1 END, created_at DESC")
	utils.LogDebug("Applied ordering by pending status and creation date")

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.LogError("Failed to count return requests: %v", err)
		utils.InternalServerError(c, "Failed to count return requests", err.Error())
		return
	}
	utils.LogDebug("Total return requests found: %d", total)

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch return requests: %v", err)
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders with return requests", len(orders))

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
				"id":                  order.ID,
				"username":            order.User.Username,
				"email":               order.User.Email,
				"status":              order.Status,
				"total_amount":        fmt.Sprintf("%.2f", order.TotalAmount),
				"discount":            fmt.Sprintf("%.2f", order.Discount),
				"coupon_discount":     fmt.Sprintf("%.2f", order.CouponDiscount),
				"delivery_charge":     fmt.Sprintf("%.2f", order.DeliveryCharge),
				"total_with_delivery": fmt.Sprintf("%.2f", order.TotalWithDelivery),
				"final_total":         fmt.Sprintf("%.2f", order.FinalTotal),
				"created_at":          order.CreatedAt.Format("2006-01-02 15:04:05"),
				"return_reason":       order.ReturnReason,
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
	utils.LogDebug("Prepared response for %d return requests", len(returnRequests))

	utils.LogInfo("Successfully retrieved %d return requests", len(returnRequests))
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

// AdminListAllReturns lists all return requests including approved and rejected ones
func AdminListAllReturns(c *gin.Context) {
	utils.LogInfo("AdminListAllReturns called")

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

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit := 10 // Fixed limit of 10 items per page
	utils.LogDebug("Pagination parameters - Page: %d, Limit: %d", page, limit)

	// Build base query with proper ordering (pending first)
	query := config.DB.
		Preload("User").
		Preload("Address").
		Preload("OrderItems").
		Where("has_item_return_requests = ? OR EXISTS (SELECT 1 FROM order_items WHERE order_items.order_id = orders.id AND order_items.return_requested = true)", true).
		Order("CASE WHEN status = '" + models.OrderStatusReturnRequested + "' THEN 0 ELSE 1 END, created_at DESC")
	utils.LogDebug("Built base query with proper ordering")

	// Get total count
	var total int64
	if err := query.Model(&models.Order{}).Count(&total).Error; err != nil {
		utils.LogError("Failed to count return requests: %v", err)
		utils.InternalServerError(c, "Failed to count return requests", err.Error())
		return
	}
	utils.LogDebug("Total return requests found: %d", total)

	// Apply pagination
	var orders []models.Order
	if err := query.Offset((page - 1) * limit).Limit(limit).Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch return requests: %v", err)
		utils.InternalServerError(c, "Failed to fetch return requests", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders with return requests", len(orders))

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
		utils.LogDebug("Processed %d return items for order %d", len(items), order.ID)

		returnRequests = append(returnRequests, gin.H{
			"id":                  order.ID,
			"username":            order.User.Username,
			"email":               order.User.Email,
			"status":              order.Status,
			"total_amount":        fmt.Sprintf("%.2f", order.TotalAmount),
			"discount":            fmt.Sprintf("%.2f", order.Discount),
			"coupon_discount":     fmt.Sprintf("%.2f", order.CouponDiscount),
			"delivery_charge":     fmt.Sprintf("%.2f", order.DeliveryCharge),
			"total_with_delivery": fmt.Sprintf("%.2f", order.TotalWithDelivery),
			"final_total":         fmt.Sprintf("%.2f", order.FinalTotal),
			"created_at":          order.CreatedAt.Format("2006-01-02 15:04:05"),
			"return_reason":       order.ReturnReason,
			"return_items":        items,
			"return_summary": gin.H{
				"total_items":    stats.total,
				"pending_items":  stats.pending,
				"approved_items": stats.approved,
				"rejected_items": stats.rejected,
				"is_all_pending": stats.pending == stats.total && stats.total > 0,
			},
		})
	}
	utils.LogDebug("Prepared response for %d return requests", len(returnRequests))

	utils.LogInfo("Successfully retrieved %d return requests", len(returnRequests))
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
