package controllers

import (
	"fmt"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// DashboardStats represents the response structure for dashboard statistics
type DashboardStats struct {
	TotalSales     string `json:"total_sales"`
	TotalOrders    int64  `json:"total_orders"`
	TotalCustomers int64  `json:"total_customers"`
	TotalProducts  int64  `json:"total_products"`
}

// SalesChartData represents the response structure for sales chart data
type SalesChartData struct {
	Labels []string `json:"labels"`
	Data   []string `json:"data"` // Changed to string array for formatted values
}

// TopSellingItem represents a top selling item (product/category/brand)
type TopSellingItem struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	TotalSales  string `json:"total_sales"` // Changed to string for formatted value
	TotalOrders int64  `json:"total_orders"`
	Quantity    int64  `json:"quantity"`
}

// GetDashboardStats returns overall dashboard statistics
func GetDashboardStats(c *gin.Context) {
	utils.LogInfo("GetDashboardStats called")

	var stats DashboardStats
	var totalSales float64

	// Get total sales
	config.DB.Model(&models.Order{}).
		Where("status != ?", models.OrderStatusCancelled).
		Select("COALESCE(SUM(final_total), 0)").
		Row().Scan(&totalSales)

	// Format total sales with 2 decimal places
	stats.TotalSales = fmt.Sprintf("%.2f", totalSales)
	utils.LogDebug("Total sales calculated: %s", stats.TotalSales)

	// Get total orders
	config.DB.Model(&models.Order{}).
		Where("status != ?", models.OrderStatusCancelled).
		Count(&stats.TotalOrders)
	utils.LogDebug("Total orders counted: %d", stats.TotalOrders)

	// Get total customers
	config.DB.Model(&models.User{}).Count(&stats.TotalCustomers)
	utils.LogDebug("Total customers counted: %d", stats.TotalCustomers)

	// Get total products
	config.DB.Model(&models.Book{}).Count(&stats.TotalProducts)
	utils.LogDebug("Total products counted: %d", stats.TotalProducts)

	utils.LogInfo("Successfully retrieved dashboard statistics")
	utils.Success(c, "Dashboard statistics retrieved successfully", stats)
}

// GetSalesChart returns sales data for charts with time-based filtering
func GetSalesChart(c *gin.Context) {
	utils.LogInfo("GetSalesChart called")

	period := c.Query("period") // yearly, monthly, weekly, daily
	if period == "" {
		period = "monthly" // default to monthly
	}
	utils.LogDebug("Generating sales chart for period: %s", period)

	var chartData SalesChartData
	var query *gorm.DB

	// Set time range based on period
	now := time.Now()
	var startTime time.Time
	var timeFormat string

	switch period {
	case "yearly":
		startTime = now.AddDate(-1, 0, 0)
		timeFormat = "2006"
		query = config.DB.Model(&models.Order{}).
			Select("DATE_TRUNC('year', created_at) as period, SUM(final_total) as total").
			Where("created_at >= ? AND status != ?", startTime, models.OrderStatusCancelled).
			Group("period").
			Order("period ASC")
		utils.LogDebug("Yearly chart - Start time: %s", startTime.Format("2006-01-02"))
	case "monthly":
		startTime = now.AddDate(0, -12, 0)
		timeFormat = "2006-01"
		query = config.DB.Model(&models.Order{}).
			Select("DATE_TRUNC('month', created_at) as period, SUM(final_total) as total").
			Where("created_at >= ? AND status != ?", startTime, models.OrderStatusCancelled).
			Group("period").
			Order("period ASC")
		utils.LogDebug("Monthly chart - Start time: %s", startTime.Format("2006-01-02"))
	case "weekly":
		startTime = now.AddDate(0, 0, -30)
		timeFormat = "2006-01-02"
		query = config.DB.Model(&models.Order{}).
			Select("DATE_TRUNC('week', created_at) as period, SUM(final_total) as total").
			Where("created_at >= ? AND status != ?", startTime, models.OrderStatusCancelled).
			Group("period").
			Order("period ASC")
		utils.LogDebug("Weekly chart - Start time: %s", startTime.Format("2006-01-02"))
	case "daily":
		startTime = now.AddDate(0, 0, -30)
		timeFormat = "2006-01-02"
		query = config.DB.Model(&models.Order{}).
			Select("DATE_TRUNC('day', created_at) as period, SUM(final_total) as total").
			Where("created_at >= ? AND status != ?", startTime, models.OrderStatusCancelled).
			Group("period").
			Order("period ASC")
		utils.LogDebug("Daily chart - Start time: %s", startTime.Format("2006-01-02"))
	default:
		utils.LogError("Invalid period specified: %s", period)
		utils.BadRequest(c, "Invalid period. Must be one of: yearly, monthly, weekly, daily", nil)
		return
	}

	// Execute query and collect results
	type Result struct {
		Period time.Time
		Total  float64
	}
	var results []Result
	if err := query.Find(&results).Error; err != nil {
		utils.LogError("Failed to fetch sales data: %v", err)
		utils.InternalServerError(c, "Failed to fetch sales data", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d data points for sales chart", len(results))

	// Format data for response
	for _, r := range results {
		chartData.Labels = append(chartData.Labels, r.Period.Format(timeFormat))
		chartData.Data = append(chartData.Data, fmt.Sprintf("%.2f", r.Total))
	}

	utils.LogInfo("Successfully generated sales chart for period %s", period)
	utils.Success(c, "Sales chart data retrieved successfully", chartData)
}

// GetTopSellingProducts returns top 10 selling products
func GetTopSellingProducts(c *gin.Context) {
	utils.LogInfo("GetTopSellingProducts called")

	type RawProduct struct {
		ID          uint
		Name        string
		TotalSales  float64
		TotalOrders int64
		Quantity    int64
	}
	var rawProducts []RawProduct

	query := config.DB.Model(&models.OrderItem{}).
		Select("books.id, books.name, SUM(order_items.total) as total_sales, COUNT(DISTINCT order_items.order_id) as total_orders, SUM(order_items.quantity) as quantity").
		Joins("JOIN books ON books.id = order_items.book_id").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.status != ?", models.OrderStatusCancelled).
		Group("books.id, books.name").
		Order("total_sales DESC").
		Limit(10)

	if err := query.Find(&rawProducts).Error; err != nil {
		utils.LogError("Failed to fetch top selling products: %v", err)
		utils.InternalServerError(c, "Failed to fetch top selling products", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d top selling products", len(rawProducts))

	// Convert to formatted response
	products := make([]TopSellingItem, len(rawProducts))
	for i, p := range rawProducts {
		products[i] = TopSellingItem{
			ID:          p.ID,
			Name:        p.Name,
			TotalSales:  fmt.Sprintf("%.2f", p.TotalSales),
			TotalOrders: p.TotalOrders,
			Quantity:    p.Quantity,
		}
	}

	utils.LogInfo("Successfully retrieved top selling products")
	utils.Success(c, "Top selling products retrieved successfully", products)
}

// GetTopSellingCategories returns top 10 selling categories
func GetTopSellingCategories(c *gin.Context) {
	utils.LogInfo("GetTopSellingCategories called")

	type RawCategory struct {
		ID          uint
		Name        string
		TotalSales  float64
		TotalOrders int64
		Quantity    int64
	}
	var rawCategories []RawCategory

	query := config.DB.Model(&models.OrderItem{}).
		Select("categories.id, categories.name, SUM(order_items.total) as total_sales, COUNT(DISTINCT order_items.order_id) as total_orders, SUM(order_items.quantity) as quantity").
		Joins("JOIN books ON books.id = order_items.book_id").
		Joins("JOIN categories ON categories.id = books.category_id").
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.status != ?", models.OrderStatusCancelled).
		Group("categories.id, categories.name").
		Order("total_sales DESC").
		Limit(10)

	if err := query.Find(&rawCategories).Error; err != nil {
		utils.LogError("Failed to fetch top selling categories: %v", err)
		utils.InternalServerError(c, "Failed to fetch top selling categories", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d top selling categories", len(rawCategories))

	// Convert to formatted response
	categories := make([]TopSellingItem, len(rawCategories))
	for i, c := range rawCategories {
		categories[i] = TopSellingItem{
			ID:          c.ID,
			Name:        c.Name,
			TotalSales:  fmt.Sprintf("%.2f", c.TotalSales),
			TotalOrders: c.TotalOrders,
			Quantity:    c.Quantity,
		}
	}

	utils.LogInfo("Successfully retrieved top selling categories")
	utils.Success(c, "Top selling categories retrieved successfully", categories)
}
