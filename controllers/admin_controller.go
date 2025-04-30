package controllers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	log.Printf("Admin login attempt for email: %s", req.Email)

	var admin models.Admin
	if err := config.DB.Where("email = ?", req.Email).First(&admin).Error; err != nil {
		log.Printf("Admin not found: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if !admin.IsActive {
		log.Printf("Admin account is inactive: %s", admin.Email)
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin account is inactive"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		log.Printf("Invalid password for admin: %s", admin.Email)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not configured"})
		return
	}

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Printf("Failed to generate token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token: " + err.Error()})
		return
	}

	log.Printf("Token generated successfully for admin: %s", admin.Email)
	log.Printf("Token: %s", tokenString)

	c.JSON(http.StatusOK, gin.H{
		"message": "Login successful",
		"status":  "success",
		"token":   tokenString,
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch users"})
		return
	}

	// Log the results
	log.Printf("Found %d users", len(users))
	for i, user := range users {
		log.Printf("User %d: ID=%d, Email=%s, CreatedAt=%v", i+1, user.ID, user.Email, user.CreatedAt)
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found in context"})
		return
	}

	adminModel, ok := admin.(models.Admin)
	if !ok {
		log.Printf("Invalid admin type in context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid admin type"})
		return
	}

	log.Printf("Admin authenticated: %s", adminModel.Email)

	// Get user ID from URL parameter
	userID := c.Param("id")
	if userID == "" {
		log.Printf("User ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	log.Printf("Processing user with ID: %s", userID)

	// Find the user
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		log.Printf("User not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user block status"})
		return
	}

	log.Printf("User %s successfully: %s", action, user.Email)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("User %s successfully", action),
		"user":    user,
	})
}

// AdminLogout handles admin logout
func AdminLogout(c *gin.Context) {
	// In a JWT-based system, logout is handled client-side by removing the token
	// We can add additional server-side cleanup if needed
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
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
	RecentOrders   []models.Order   `json:"recent_orders"`
	TopBooks       []models.Book    `json:"top_books"`
	NavigationMenu []NavigationItem `json:"navigation_menu"`
}

// NavigationItem represents a menu item in the dashboard navigation
type NavigationItem struct {
	Title    string           `json:"title"`
	Path     string           `json:"path"`
	Icon     string           `json:"icon"`
	Children []NavigationItem `json:"children,omitempty"`
}

// GetDashboardOverview returns the admin dashboard overview
func GetDashboardOverview(c *gin.Context) {
	var overview DashboardOverview

	// Get total sales count
	config.DB.Model(&models.Order{}).Count(&overview.TotalOrders)

	// Get total revenue
	config.DB.Model(&models.Order{}).Select("COALESCE(SUM(total_amount), 0)").Scan(&overview.TotalRevenue)

	// Get total customers
	config.DB.Model(&models.User{}).Count(&overview.TotalCustomers)

	// Get recent orders (last 5)
	config.DB.Preload("User").Order("created_at desc").Limit(5).Find(&overview.RecentOrders)

	// Get top books (by views)
	config.DB.Preload("Category").Order("views desc").Limit(5).Find(&overview.TopBooks)

	// Define navigation menu
	overview.NavigationMenu = []NavigationItem{
		{
			Title: "Dashboard",
			Path:  "/admin/dashboard",
			Icon:  "dashboard",
		},
		{
			Title: "Book Management",
			Path:  "/admin/books",
			Icon:  "book",
			Children: []NavigationItem{
				{Title: "All Books", Path: "/admin/books", Icon: "list"},
				{Title: "Add Book", Path: "/admin/books/new", Icon: "add"},
				{Title: "Categories", Path: "/admin/categories", Icon: "category"},
			},
		},
		{
			Title: "Order Management",
			Path:  "/admin/orders",
			Icon:  "shopping_cart",
			Children: []NavigationItem{
				{Title: "All Orders", Path: "/admin/orders", Icon: "list"},
				{Title: "Pending Orders", Path: "/admin/orders/pending", Icon: "pending"},
			},
		},
		{
			Title: "Customer Management",
			Path:  "/admin/users",
			Icon:  "people",
			Children: []NavigationItem{
				{Title: "All Customers", Path: "/admin/users", Icon: "list"},
				{Title: "Blocked Users", Path: "/admin/users/blocked", Icon: "block"},
			},
		},
		{
			Title: "Reports",
			Path:  "/admin/reports",
			Icon:  "analytics",
			Children: []NavigationItem{
				{Title: "Sales Report", Path: "/admin/reports/sales", Icon: "trending_up"},
				{Title: "Customer Report", Path: "/admin/reports/customers", Icon: "people"},
			},
		},
		{
			Title: "Settings",
			Path:  "/admin/settings",
			Icon:  "settings",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Dashboard overview loaded successfully",
		"status":   "success",
		"overview": overview,
	})
}
