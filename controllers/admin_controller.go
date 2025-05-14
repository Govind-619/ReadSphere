package controllers

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// AdminLoginRequest represents the admin login request
type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AdminLogin handles admin authentication
func AdminLogin(c *gin.Context) {
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	log.Printf("Admin login attempt for email: %s", req.Email)

	var admin models.Admin
	if err := config.DB.Where("email = ?", req.Email).First(&admin).Error; err != nil {
		log.Printf("Admin not found: %v", err)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	if !admin.IsActive {
		log.Printf("Admin account is inactive: %s", admin.Email)
		utils.Forbidden(c, "Admin account is inactive")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		log.Printf("Invalid password for admin: %s", admin.Email)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	// Update last login
	admin.LastLogin = time.Now()
	config.DB.Save(&admin)

	// Generate JWT token with simpler claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": admin.ID,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	log.Printf("JWT Secret length for token generation: %d", len(jwtSecret))

	if jwtSecret == "" {
		log.Printf("JWT secret not configured for token generation")
		utils.InternalServerError(c, "JWT secret not configured", nil)
		return
	}

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}

	log.Printf("Token generated successfully for admin: %s", admin.Email)

	utils.Success(c, "Login successful", gin.H{
		"token": tokenString,
		"admin": gin.H{
			"id":        admin.ID,
			"email":     admin.Email,
			"firstName": admin.FirstName,
			"lastName":  admin.LastName,
		},
	})
}

// UserListRequest represents the request parameters for user listing
type UserListRequest struct {
	Page   int    `form:"page" binding:"min=1"`
	Limit  int    `form:"limit" binding:"min=1,max=100"`
	Search string `form:"search"`
	SortBy string `form:"sort_by"`
	Order  string `form:"order" binding:"oneof=asc desc"`
}

// GetUsers handles user listing with search, pagination, and sorting
func GetUsers(c *gin.Context) {
	log.Printf("GetUsers called")

	var req UserListRequest

	// Set default values for query parameters
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))
	req.SortBy = c.DefaultQuery("sort_by", "created_at")
	req.Order = c.DefaultQuery("order", "desc")
	req.Search = c.Query("search")

	// Set defaults
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Order == "" {
		req.Order = "desc"
	}

	query := config.DB.Model(&models.User{}).Preload("Addresses")

	// Apply search with improved logging
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		log.Printf("Applying search with term: %s", req.Search)
		query = query.Where(
			"email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	} else {
		log.Printf("No search term provided, returning all users")
	}

	// Apply sorting
	switch req.SortBy {
	case "email":
		query = query.Order(fmt.Sprintf("email %s", req.Order))
	case "name":
		query = query.Order(fmt.Sprintf("first_name %s, last_name %s", req.Order, req.Order))
	case "created_at":
		query = query.Order(fmt.Sprintf("created_at %s", req.Order))
	default:
		query = query.Order(fmt.Sprintf("created_at %s", req.Order))
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Apply pagination
	offset := (req.Page - 1) * req.Limit
	query = query.Offset(offset).Limit(req.Limit)

	// Add debug logging
	log.Printf("Query parameters - Page: %d, Limit: %d, Order: %s, SortBy: %s, Search: %s",
		req.Page, req.Limit, req.Order, req.SortBy, req.Search)
	log.Printf("SQL Query: %v", query.Statement.SQL.String())

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		log.Printf("Failed to fetch users: %v", err)
		utils.InternalServerError(c, "Failed to fetch users", err.Error())
		return
	}

	// Log the results
	log.Printf("Found %d users", len(users))
	for i, user := range users {
		log.Printf("User %d: ID=%d, Email=%s, CreatedAt=%v", i+1, user.ID, user.Email, user.CreatedAt)
	}

	// Create clean response without sensitive data
	cleanUsers := make([]gin.H, len(users))
	for i, user := range users {
		cleanUsers[i] = gin.H{
			"id":            user.ID,
			"username":      user.Username,
			"email":         user.Email,
			"first_name":    user.FirstName,
			"last_name":     user.LastName,
			"is_blocked":    user.IsBlocked,
			"is_verified":   user.IsVerified,
			"created_at":    user.CreatedAt,
			"last_login":    user.LastLoginAt,
			"address_count": len(user.Addresses),
		}
	}

	utils.Success(c, "Users retrieved successfully", gin.H{
		"users": cleanUsers,
		"pagination": gin.H{
			"total":       total,
			"page":        req.Page,
			"limit":       req.Limit,
			"total_pages": (total + int64(req.Limit) - 1) / int64(req.Limit),
		},
		"search": gin.H{
			"term": req.Search,
		},
	})
}

// BlockUser handles blocking/unblocking a user
func BlockUser(c *gin.Context) {
	log.Printf("BlockUser called")

	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		log.Printf("Admin not found in context")
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		log.Printf("Invalid admin type in context")
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	// Get user ID from URL parameter
	userID := c.Param("id")
	if userID == "" {
		log.Printf("User ID not provided")
		utils.BadRequest(c, "User ID is required", nil)
		return
	}

	log.Printf("Processing user with ID: %s", userID)

	// Find the user
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		log.Printf("User not found: %v", err)
		utils.NotFound(c, "User not found")
		return
	}

	// Toggle the block status
	newBlockStatus := !user.IsBlocked
	action := "blocked"
	if !newBlockStatus {
		action = "unblocked"
	}

	// Update only the is_blocked field and updated_at timestamp
	updates := map[string]interface{}{
		"is_blocked": newBlockStatus,
		"updated_at": time.Now(),
	}

	// Update the user
	if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
		log.Printf("Failed to update user block status: %v", err)
		utils.InternalServerError(c, "Failed to update user block status", err.Error())
		return
	}

	log.Printf("User %s successfully: %s", action, user.Email)
	utils.Success(c, fmt.Sprintf("User %s successfully", action), gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"is_blocked": user.IsBlocked,
		},
	})
}

// AdminLogout handles admin logout
func AdminLogout(c *gin.Context) {
	// In a JWT-based system, logout is handled client-side by removing the token
	// We can add additional server-side cleanup if needed
	utils.Success(c, "Logged out successfully", nil)
}

// CreateSampleAdmin creates a sample admin user
func CreateSampleAdmin() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(os.Getenv("ADMIN_PASSWORD")), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	admin := models.Admin{
		Email:     os.Getenv("ADMIN_EMAIL"),
		Password:  string(hashedPassword),
		FirstName: os.Getenv("ADMIN_FIRST_NAME"),
		LastName:  os.Getenv("ADMIN_LAST_NAME"),
		IsActive:  true,
	}

	return config.DB.FirstOrCreate(&admin, models.Admin{Email: admin.Email}).Error
}

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
	log.Printf("GetDashboardOverview called")

	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		log.Printf("Admin not found in context")
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		log.Printf("Invalid admin type in context")
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	// Get total sales (completed orders)
	var totalSales int64
	if err := config.DB.Model(&models.Order{}).Where("status = ?", "Delivered").Count(&totalSales).Error; err != nil {
		log.Printf("Failed to get total sales: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

	// Get total orders
	var totalOrders int64
	if err := config.DB.Model(&models.Order{}).Count(&totalOrders).Error; err != nil {
		log.Printf("Failed to get total orders: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

	// Get total revenue from completed orders
	var totalRevenue float64
	if err := config.DB.Model(&models.OrderItem{}).
		Joins("JOIN orders ON orders.id = order_items.order_id").
		Where("orders.status = ?", "Delivered").
		Select("COALESCE(SUM(order_items.quantity * order_items.price), 0)").
		Scan(&totalRevenue).Error; err != nil {
		log.Printf("Failed to get total revenue: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

	// Get total customers
	var totalCustomers int64
	if err := config.DB.Model(&models.User{}).Count(&totalCustomers).Error; err != nil {
		log.Printf("Failed to get total customers: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

	// Get recent orders (last 3)
	var recentOrders []models.Order
	if err := config.DB.Preload("User").
		Preload("OrderItems.Book").
		Order("created_at desc").
		Limit(3).
		Find(&recentOrders).Error; err != nil {
		log.Printf("Failed to get recent orders: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

	// Get top books by views (top 3)
	var topBooks []models.Book
	if err := config.DB.Order("views desc").Limit(3).Find(&topBooks).Error; err != nil {
		log.Printf("Failed to get top books: %v", err)
		utils.InternalServerError(c, "Failed to get dashboard data", err.Error())
		return
	}

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

	overview := DashboardOverview{
		TotalSales:     totalSales,
		TotalOrders:    totalOrders,
		TotalRevenue:   totalRevenue,
		TotalCustomers: totalCustomers,
		RecentOrders:   recentOrdersOverview,
		TopBooks:       topBooksOverview,
		NavigationMenu: navigationMenu,
	}

	utils.Success(c, "Dashboard data retrieved successfully", gin.H{
		"overview": overview,
	})
}
