package controllers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

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
	utils.LogInfo("GetUsers called")

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
	utils.LogDebug("Query parameters set - Page: %d, Limit: %d, SortBy: %s, Order: %s", req.Page, req.Limit, req.SortBy, req.Order)

	query := config.DB.Model(&models.User{}).Preload("Addresses")

	// Apply search with improved logging
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		utils.LogDebug("Applying search with term: %s", req.Search)
		query = query.Where(
			"email ILIKE ? OR first_name ILIKE ? OR last_name ILIKE ?",
			searchTerm, searchTerm, searchTerm,
		)
	} else {
		utils.LogDebug("No search term provided, returning all users")
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
	utils.LogDebug("Applied sorting - SortBy: %s, Order: %s", req.SortBy, req.Order)

	// Get total count
	var total int64
	query.Count(&total)
	utils.LogDebug("Total users count: %d", total)

	// Apply pagination
	offset := (req.Page - 1) * req.Limit
	query = query.Offset(offset).Limit(req.Limit)
	utils.LogDebug("Applied pagination - Offset: %d, Limit: %d", offset, req.Limit)

	var users []models.User
	if err := query.Find(&users).Error; err != nil {
		utils.LogError("Failed to fetch users: %v", err)
		utils.InternalServerError(c, "Failed to fetch users", err.Error())
		return
	}

	// Log the results
	utils.LogDebug("Found %d users", len(users))
	for i, user := range users {
		utils.LogDebug("User %d: ID=%d, Email=%s, CreatedAt=%v", i+1, user.ID, user.Email, user.CreatedAt)
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
	utils.LogDebug("Prepared clean user data for response")

	utils.LogInfo("Successfully retrieved %d users", len(users))
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
	utils.LogInfo("BlockUser called")

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

	// Get user ID from URL parameter
	userID := c.Param("id")
	if userID == "" {
		utils.LogError("User ID not provided")
		utils.BadRequest(c, "User ID is required", nil)
		return
	}

	utils.LogDebug("Processing user with ID: %s", userID)

	// Find the user
	var user models.User
	if err := config.DB.First(&user, userID).Error; err != nil {
		utils.LogError("User not found: %v", err)
		utils.NotFound(c, "User not found")
		return
	}

	// Toggle the block status
	newBlockStatus := !user.IsBlocked
	action := "blocked"
	if !newBlockStatus {
		action = "unblocked"
	}
	utils.LogDebug("Toggling block status for user %s to %v", user.Email, newBlockStatus)

	// Update only the is_blocked field and updated_at timestamp
	updates := map[string]interface{}{
		"is_blocked": newBlockStatus,
		"updated_at": time.Now(),
	}

	// Update the user
	if err := config.DB.Model(&user).Updates(updates).Error; err != nil {
		utils.LogError("Failed to update user block status: %v", err)
		utils.InternalServerError(c, "Failed to update user block status", err.Error())
		return
	}

	utils.LogInfo("User %s successfully %s", user.Email, action)
	utils.Success(c, fmt.Sprintf("User %s successfully", action), gin.H{
		"user": gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"username":   user.Username,
			"is_blocked": user.IsBlocked,
		},
	})
}
