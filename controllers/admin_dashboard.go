package controllers

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// DashboardOverview represents the admin dashboard overview data
type DashboardOverview struct {
	TotalSales     int64            `json:"total_sales"`
	TotalOrders    int64            `json:"total_orders"`
	TotalRevenue   float64          `json:"total_revenue"`
	TotalCustomers int64            `json:"total_customers"`
	RecentOrders   []OrderOverview  `json:"recent_orders"`
	TopBooks       []BookOverview   `json:"top_books"`
	NavigationMenu []NavigationItem `json:"navigation_menu"`
}

// OrderOverview represents simplified order data for the dashboard
type OrderOverview struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Status    string    `json:"status"`
	Total     float64   `json:"total"`
	CreatedAt time.Time `json:"created_at"`
	ItemCount int       `json:"item_count"`
}

// BookOverview represents simplified book data for the dashboard
type BookOverview struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Views    int    `json:"views"`
}

// NavigationItem represents a menu item in the dashboard navigation
type NavigationItem struct {
	Name     string           `json:"name"`
	Path     string           `json:"path"`
	Icon     string           `json:"icon"`
	Children []NavigationItem `json:"children,omitempty"`
}

// GetDashboardOverview returns overview data for admin dashboard
func GetDashboardOverview(c *gin.Context) {
	utils.LogInfo("GetDashboardOverview called")

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

	// Get total sales (completed orders)
	var totalSales int64
	if err := config.DB.Model(&models.Order{}).Where("status = ?", "Delivered").Count(&totalSales).Error; err != nil {
		utils.LogError("Failed to get total sales: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved total sales: %d", totalSales)

	// Get total orders
	var totalOrders int64
	if err := config.DB.Model(&models.Order{}).Count(&totalOrders).Error; err != nil {
		utils.LogError("Failed to get total orders: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved total orders: %d", totalOrders)

	// Get total revenue from completed orders
	var totalRevenue float64
	if err := config.DB.Model(&models.OrderItem{}).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.status = ?", "Delivered").
		Select("COALESCE(SUM(order_items.quantity * order_items.price), 0)").
		Scan(&totalRevenue).Error; err != nil {
		utils.LogError("Failed to get total revenue: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved total revenue: %.2f", totalRevenue)

	// Get total customers
	var totalCustomers int64
	if err := config.DB.Model(&models.User{}).Count(&totalCustomers).Error; err != nil {
		utils.LogError("Failed to get total customers: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved total customers: %d", totalCustomers)

	// Get recent orders (last 3)
	var recentOrders []models.Order
	if err := config.DB.Preload("User").
		Preload("OrderItems.Book").
		Order("created_at desc").
		Limit(3).
		Find(&recentOrders).Error; err != nil {
		utils.LogError("Failed to get recent orders: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d recent orders", len(recentOrders))

	// Get top books by views (top 3)
	var topBooks []models.Book
	if err := config.DB.Order("views desc").Limit(3).Find(&topBooks).Error; err != nil {
		utils.LogError("Failed to get top books: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d top books", len(topBooks))

	// Prepare recent orders overview
	recentOrdersOverview := make([]OrderOverview, 0, len(recentOrders))
	for _, order := range recentOrders {
		recentOrdersOverview = append(recentOrdersOverview, OrderOverview{
			ID:        order.ID,
			Username:  order.User.Username,
			Status:    order.Status,
			Total:     order.FinalTotal,
			CreatedAt: order.CreatedAt,
			ItemCount: len(order.OrderItems),
		})
	}
	utils.LogDebug("Prepared recent orders overview")

	// Prepare top books overview
	topBooksOverview := make([]BookOverview, 0, len(topBooks))
	for _, book := range topBooks {
		topBooksOverview = append(topBooksOverview, BookOverview{
			ID:       book.ID,
			Name:     book.Name,
			Category: book.Category.Name,
			Views:    book.Views,
		})
	}
	utils.LogDebug("Prepared top books overview")

	// Navigation menu items
	navigationMenu := []NavigationItem{
		{Name: "Dashboard", Path: "/admin/dashboard", Icon: "dashboard"},
		{Name: "Orders", Path: "/admin/orders", Icon: "shopping_cart"},
		{Name: "Products", Path: "/admin/books", Icon: "book"},
		{Name: "Categories", Path: "/admin/categories", Icon: "category"},
		{Name: "Customers", Path: "/admin/users", Icon: "people"},
		{Name: "Reports", Path: "/admin/reports", Icon: "assessment"},
		{Name: "Settings", Path: "/admin/settings", Icon: "settings"},
	}
	utils.LogDebug("Prepared navigation menu")

	overview := DashboardOverview{
		TotalSales:     totalSales,
		TotalOrders:    totalOrders,
		TotalRevenue:   totalRevenue,
		TotalCustomers: totalCustomers,
		RecentOrders:   recentOrdersOverview,
		TopBooks:       topBooksOverview,
		NavigationMenu: navigationMenu,
	}

	utils.LogInfo("Successfully retrieved dashboard overview for admin: %s", adminModel.Email)
	utils.Success(c, "Dashboard data retrieved successfully", gin.H{
		"overview": overview,
	})
}
