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
	// Get period from query
	period := c.DefaultQuery("period", "day") // day, week, month, custom

	// Calculate date ranges based on period
	now := time.Now()
	var startDate, endDate time.Time

	switch period {
	case "day":
		startDate = now.Truncate(24 * time.Hour)
		endDate = startDate.Add(24 * time.Hour)
	case "week":
		startDate = now.AddDate(0, 0, -7).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	case "month":
		startDate = now.AddDate(0, 0, -30).Truncate(24 * time.Hour)
		endDate = now.Add(24 * time.Hour)
	case "custom":
		// Parse custom date range
		startDateStr := c.Query("start_date")
		endDateStr := c.Query("end_date")

		if startDateStr == "" || endDateStr == "" {
			utils.BadRequest(c, "Missing date range", "Both start_date and end_date are required for custom period")
			return
		}

		var err error
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			utils.BadRequest(c, "Invalid start date", "Start date must be in YYYY-MM-DD format")
			return
		}

		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			utils.BadRequest(c, "Invalid end date", "End date must be in YYYY-MM-DD format")
			return
		}

		// Add one day to end date to include the entire end date
		endDate = endDate.Add(24 * time.Hour)

		// Validate date range
		if endDate.Before(startDate) {
			utils.BadRequest(c, "Invalid date range", "End date must be after start date")
			return
		}

		if endDate.Sub(startDate) > 90*24*time.Hour {
			utils.BadRequest(c, "Invalid date range", "Date range cannot exceed 90 days")
			return
		}
	default:
		utils.BadRequest(c, "Invalid period", "Period must be day, week, month, or custom")
		return
	}

	// Query orders within date range
	var orders []models.Order
	query := config.DB.Where("created_at >= ? AND created_at < ?", startDate, endDate).
		Preload("User").
		Preload("OrderItems.Book").
		Order("created_at DESC")

	if err := query.Find(&orders).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch orders", err.Error())
		return
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
		if order.Status == models.OrderStatusDelivered {
			summary.TotalSales++
			summary.TotalRevenue += order.TotalAmount
			summary.TotalDiscounts += order.Discount + order.CouponDiscount
			customerSet[order.UserID] = true

			for _, item := range order.OrderItems {
				summary.TotalItems += item.Quantity
			}
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

	// Format sales data for response
	var salesData []gin.H
	for _, order := range orders {
		if order.Status == models.OrderStatusDelivered {
			salesData = append(salesData, gin.H{
				"order_id":      order.ID,
				"date":          order.CreatedAt.Format("2006-01-02"),
				"customer_name": order.User.Username,
				"items":         len(order.OrderItems),
				"total":         math.Round(order.TotalAmount*100) / 100,
				"discount":      math.Round((order.Discount+order.CouponDiscount)*100) / 100,
				"net_amount":    math.Round((order.TotalAmount-order.Discount-order.CouponDiscount)*100) / 100,
				"payment_mode":  order.PaymentMethod,
			})
		}
	}

	utils.Success(c, "Sales report generated successfully", gin.H{
		"period": gin.H{
			"type":       period,
			"start_date": startDate.Format("2006-01-02"),
			"end_date":   endDate.Add(-24 * time.Hour).Format("2006-01-02"), // Subtract one day since we added it for query
		},
		"summary": summary,
		"sales":   salesData,
	})
}
