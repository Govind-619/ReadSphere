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

// CategoryBlockRequest represents the block/unblock request
type CategoryBlockRequest struct {
	Blocked bool `json:"blocked"`
}

// ToggleCategoryBlock handles blocking/unblocking of categories
func ToggleCategoryBlock(c *gin.Context) {
	utils.LogInfo("ToggleCategoryBlock called")

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

	// Validate and parse category ID
	categoryID := c.Param("id")
	if categoryID == "" {
		utils.LogError("Category ID not provided")
		utils.BadRequest(c, "Category ID is required", nil)
		return
	}

	id, err := strconv.ParseUint(categoryID, 10, 32)
	if err != nil {
		utils.LogError("Invalid category ID format: %v", err)
		utils.BadRequest(c, "Invalid category ID format", "Category ID must be a valid number")
		return
	}
	utils.LogDebug("Processing category ID: %d", id)

	// Parse request body
	var req CategoryBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", "Invalid request format")
		return
	}
	utils.LogDebug("Received block request - Blocked: %v", req.Blocked)

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process request", nil)
		return
	}
	utils.LogDebug("Started database transaction")

	// Get category
	var category models.Category
	if err := tx.First(&category, id).Error; err != nil {
		tx.Rollback()
		utils.LogError("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}
	utils.LogDebug("Found category: %s", category.Name)

	// Update blocked status
	if err := tx.Model(&category).Updates(map[string]interface{}{
		"blocked":    req.Blocked,
		"updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update category block status: %v", err)
		utils.InternalServerError(c, "Failed to update category", err.Error())
		return
	}
	utils.LogDebug("Updated category block status to: %v", req.Blocked)

	// If blocking, also block all books in this category
	if req.Blocked {
		if err := tx.Model(&models.Book{}).Where("category_id = ?", id).Updates(map[string]interface{}{
			"blocked":    true,
			"updated_at": time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to block category books: %v", err)
			utils.InternalServerError(c, "Failed to block category books", err.Error())
			return
		}
		utils.LogDebug("Blocked all books in category")
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to save changes", err.Error())
		return
	}
	utils.LogDebug("Successfully committed transaction")

	action := "blocked"
	if !req.Blocked {
		action = "unblocked"
	}

	utils.LogInfo("Category %s successfully: %s", action, category.Name)
	utils.Success(c, fmt.Sprintf("Category %s successfully", action), gin.H{
		"category": gin.H{
			"id":      category.ID,
			"blocked": req.Blocked,
		},
	})
}
