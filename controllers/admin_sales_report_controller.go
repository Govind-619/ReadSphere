package controllers

import (
	"math"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// Admin: Generate sales report with filters and summary
func GenerateSalesReport(c *gin.Context) {
	utils.LogInfo("GenerateSalesReport called")

	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month, custom
	utils.LogDebug("Generating sales report for period: %s", period)

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		// For daily report, include all of today
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02 15:04:05"), endDate.Format("2006-01-02 15:04:05"))
	case "week":
		// For weekly report, include last 7 days including today
		endDate = time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, now.Location())
		startDate = endDate.AddDate(0, 0, -6) // 7 days including today
		startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02 15:04:05"), endDate.Format("2006-01-02 15:04:05"))
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
		utils.LogDebug("Date range: %s to %s", startDate.Format("2006-01-02 15:04:05"), endDate.Format("2006-01-02 15:04:05"))
	case "custom":
		// Parse custom date range
		startDateStr := c.Query("start_date")
		endDateStr := c.Query("end_date")

		if startDateStr == "" || endDateStr == "" {
			utils.LogError("Missing date range parameters")
			utils.BadRequest(c, "Missing date range", "Both start_date and end_date are required for custom period")
			return
		}

		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			utils.LogError("Invalid start date format: %v", err)
			utils.BadRequest(c, "Invalid start date", "Start date must be in YYYY-MM-DD format")
			return
		}

		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			utils.LogError("Invalid end date format: %v", err)
			utils.BadRequest(c, "Invalid end date", "End date must be in YYYY-MM-DD format")
			return
		}

		// Add one day to end date to include the entire end date
		endDate = endDate.Add(24 * time.Hour)
		utils.LogDebug("Custom date range: %s to %s", startDate.Format("2006-01-02 15:04:05"), endDate.Format("2006-01-02 15:04:05"))

		// Validate date range
		if endDate.Before(startDate) {
			utils.LogError("Invalid date range: end date before start date")
			utils.BadRequest(c, "Invalid date range", "End date must be after start date")
			return
		}

		if endDate.Sub(startDate) > 90*24*time.Hour {
			utils.LogError("Date range exceeds 90 days: %s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
			utils.BadRequest(c, "Invalid date range", "Date range cannot exceed 90 days")
			return
		}
	default:
		utils.LogError("Invalid period specified: %s", period)
		utils.BadRequest(c, "Invalid period", "Period must be day, week, month, or custom")
		return
	}

	// Query orders within date range
	var orders []models.Order
	query := config.DB.Where("created_at >= ? AND created_at <= ?", startDate, endDate).
		Preload("User").
		Preload("OrderItems.Book").
		Order("created_at DESC")

	if err := query.Find(&orders).Error; err != nil {
		utils.LogError("Failed to fetch orders: %v", err)
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d orders for the period", len(orders))

	// Debug log for order statuses
	for _, order := range orders {
		utils.LogDebug("Order ID: %d, Status: %s, Created At: %s, Total Amount: %.2f",
			order.ID,
			order.Status,
			order.CreatedAt.Format("2006-01-02 15:04:05"),
			order.TotalAmount)
	}

	// Calculate summary statistics
	var summary struct {
		TotalSales      int     `json:"total_sales"`
		TotalRevenue    float64 `json:"total_revenue"`
		TotalItems      int     `json:"total_items"`
		TotalCustomers  int     `json:"total_customers"`
		TotalDiscounts  float64 `json:"total_discounts"`
		TotalRefunds    float64 `json:"total_refunds"`
		NetRevenue      float64 `json:"net_revenue"`
		AverageOrderVal float64 `json:"average_order_value"`
	}

	customerSet := make(map[uint]bool)
	for _, order := range orders {
		// Include all orders in the summary, not just delivered ones
		summary.TotalSales++
		summary.TotalRevenue += order.TotalAmount
		summary.TotalDiscounts += order.Discount + order.CouponDiscount
		customerSet[order.UserID] = true

		for _, item := range order.OrderItems {
			summary.TotalItems += item.Quantity
		}

		if order.Status == models.OrderStatusRefunded || order.Status == models.OrderStatusReturnCompleted {
			summary.TotalRefunds += order.RefundAmount
		}
	}

	summary.TotalCustomers = len(customerSet)
	if summary.TotalSales > 0 {
		summary.AverageOrderVal = math.Round((summary.TotalRevenue/float64(summary.TotalSales))*100) / 100
	}
	summary.NetRevenue = math.Round((summary.TotalRevenue-summary.TotalDiscounts-summary.TotalRefunds)*100) / 100

	// Format all monetary values to 2 decimal places
	summary.TotalRevenue = math.Round(summary.TotalRevenue*100) / 100
	summary.TotalDiscounts = math.Round(summary.TotalDiscounts*100) / 100
	summary.TotalRefunds = math.Round(summary.TotalRefunds*100) / 100

	utils.LogDebug("Summary calculated - Sales: %d, Revenue: %.2f, Items: %d, Customers: %d",
		summary.TotalSales, summary.TotalRevenue, summary.TotalItems, summary.TotalCustomers)

	// Format sales data for response
	var salesData []gin.H
	for _, order := range orders {
		// Include all orders in the sales data
		salesData = append(salesData, gin.H{
			"order_id":      order.ID,
			"date":          order.CreatedAt.Format("2006-01-02 15:04:05"),
			"customer_name": order.User.Username,
			"items":         len(order.OrderItems),
			"total":         math.Round(order.TotalAmount*100) / 100,
			"discount":      math.Round((order.Discount+order.CouponDiscount)*100) / 100,
			"net_amount":    math.Round((order.TotalAmount-order.Discount-order.CouponDiscount)*100) / 100,
			"payment_mode":  order.PaymentMethod,
			"status":        order.Status,
		})
	}

	utils.LogInfo("Successfully generated sales report for period %s", period)
	utils.Success(c, "Sales report generated successfully", gin.H{
		"period": gin.H{
			"type":       period,
			"start_date": startDate.Format("2006-01-02 15:04:05"),
			"end_date":   endDate.Format("2006-01-02 15:04:05"),
		},
		"summary": summary,
		"sales":   salesData,
	})
}
