package controllers

import (
	"net/http"
	"time"
	"math"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// Admin: Generate sales report with filters and summary
func GenerateSalesReport(c *gin.Context) {
	// Parse filters
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")
	period := c.DefaultQuery("period", "custom") // day, week, month, custom

	var startDate, endDate time.Time
	var err error
	if period != "custom" {
		today := time.Now()
		switch period {
		case "day":
			startDate = today.Truncate(24 * time.Hour)
			endDate = startDate.Add(24 * time.Hour)
		case "week":
			weekday := int(today.Weekday())
			if weekday == 0 {
				weekday = 7
			}
			startDate = today.AddDate(0, 0, -weekday+1).Truncate(24 * time.Hour)
			endDate = startDate.AddDate(0, 0, 7)
		case "month":
			startDate = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, today.Location())
			endDate = startDate.AddDate(0, 1, 0)
		}
	} else {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date"})
			return
		}
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date"})
			return
		}
	}

	db := config.DB
	var orders []models.Order
	db = db.Where("created_at >= ? AND created_at < ?", startDate, endDate).Preload("User")
	db.Find(&orders)

	totalSalesCount := len(orders)
	totalOrderAmount := 0.0
	totalDiscount := 0.0
	totalCoupon := 0.0
	for _, order := range orders {
		totalOrderAmount += order.TotalAmount
		totalDiscount += order.Discount
		totalCoupon += order.CouponDiscount
	}

	// Prepare minimal order data for report
	type OrderReport struct {
		OrderID       uint   `json:"order_id"`
		UserID        uint   `json:"user_id"`
		UserName      string `json:"user_name"`
		UserEmail     string `json:"user_email"`
		TotalAmount   float64 `json:"total_amount"`
		PaymentMethod string `json:"payment_method"`
		Status        string `json:"status"`
	}
	var orderReports []OrderReport
	for _, order := range orders {
		orderReports = append(orderReports, OrderReport{
			OrderID:       order.ID,
			UserID:        order.UserID,
			UserName:      order.User.Username,
			UserEmail:     order.User.Email,
			TotalAmount:   math.Round(order.TotalAmount*100) / 100,
			PaymentMethod: order.PaymentMethod,
			Status:        order.Status,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"sales_count":     totalSalesCount,
		"order_amount":    math.Round(totalOrderAmount*100) / 100,
		"discount_amount": math.Round(totalDiscount*100) / 100,
		"coupon_amount":   math.Round(totalCoupon*100) / 100,
		"orders":          orderReports,
	})
}
