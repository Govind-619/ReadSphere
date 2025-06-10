package controllers

import (
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// CategoryRequest represents the category creation/update request
type CategoryRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"required,min=10,max=500"`
}

// CreateCategory handles category creation
func CreateCategory(c *gin.Context) {
	utils.LogInfo("CreateCategory called")

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

	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}
	utils.LogDebug("Received category creation request - Name: %s", req.Name)

	// Check if category with same name already exists
	var existingCategory models.Category
	if err := config.DB.Where("name = ?", req.Name).First(&existingCategory).Error; err == nil {
		utils.LogError("Category with name %s already exists", req.Name)
		utils.Conflict(c, "A category with this name already exists", nil)
		return
	}
	utils.LogDebug("No existing category found with name: %s", req.Name)

	category := models.Category{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&category).Error; err != nil {
		utils.LogError("Failed to create category: %v", err)
		utils.InternalServerError(c, "Failed to create category", err.Error())
		return
	}

	utils.LogInfo("Category created successfully: %s", category.Name)
	utils.Success(c, "Category created successfully", gin.H{
		"category": gin.H{
			"id":          category.ID,
			"name":        category.Name,
			"description": category.Description,
		},
	})
}

// UpdateCategory handles category updates
func UpdateCategory(c *gin.Context) {
	utils.LogInfo("UpdateCategory called")

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

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, id).Error; err != nil {
		utils.LogError("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}
	utils.LogDebug("Found category: %s", category.Name)

	// Parse and validate request body
	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"name":        "Name is required and must be between 2 and 100 characters",
			"description": "Description is required and must be between 10 and 500 characters",
		})
		return
	}
	utils.LogDebug("Received category update request - Name: %s", req.Name)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process update", nil)
		return
	}
	utils.LogDebug("Started database transaction")

	// Check for duplicate name excluding current category
	var existingCategory models.Category
	if err := tx.Where("name ILIKE ? AND id != ?", req.Name, id).First(&existingCategory).Error; err == nil {
		tx.Rollback()
		utils.LogError("Duplicate category name found: %s", req.Name)
		utils.Conflict(c, "Category name already exists", "Please choose a different name")
		return
	}
	utils.LogDebug("No duplicate category name found")

	// Update category fields
	updates := map[string]interface{}{
		"name":        strings.TrimSpace(req.Name),
		"description": strings.TrimSpace(req.Description),
		"updated_at":  time.Now(),
	}

	// Apply updates
	if err := tx.Model(&category).Updates(updates).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update category: %v", err)
		utils.InternalServerError(c, "Failed to update category", err.Error())
		return
	}
	utils.LogDebug("Updated category fields")

	// Get book count
	var bookCount int64
	if err := tx.Model(&models.Book{}).Where("category_id = ?", id).Count(&bookCount).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to get book count: %v", err)
		utils.InternalServerError(c, "Failed to get category details", err.Error())
		return
	}
	utils.LogDebug("Retrieved book count: %d", bookCount)

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to save changes", err.Error())
		return
	}
	utils.LogDebug("Successfully committed transaction")

	utils.LogInfo("Category updated successfully: %s", category.Name)
	utils.Success(c, "Category updated successfully", gin.H{
		"category": gin.H{
			"id":          category.ID,
			"name":        category.Name,
			"description": category.Description,
		},
	})
}

// DeleteCategory handles category deletion
func DeleteCategory(c *gin.Context) {
	utils.LogInfo("DeleteCategory called")

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

	categoryID := c.Param("id")
	if categoryID == "" {
		utils.LogError("Category ID not provided")
		utils.BadRequest(c, "Category ID is required", nil)
		return
	}
	utils.LogDebug("Processing category ID: %s", categoryID)

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		utils.LogError("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}
	utils.LogDebug("Found category: %s", category.Name)

	// Check if category has any books
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("category_id = ?", categoryID).Count(&bookCount).Error; err != nil {
		utils.LogError("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to check category usage", err.Error())
		return
	}
	utils.LogDebug("Category has %d books", bookCount)

	if bookCount > 0 {
		utils.LogError("Cannot delete category with %d books", bookCount)
		utils.BadRequest(c, "Cannot delete category that has books associated with it", gin.H{
			"book_count": bookCount,
		})
		return
	}

	if err := config.DB.Delete(&category).Error; err != nil {
		utils.LogError("Failed to delete category: %v", err)
		utils.InternalServerError(c, "Failed to delete category", err.Error())
		return
	}

	utils.LogInfo("Category deleted successfully: %s", category.Name)
	utils.Success(c, "Category deleted successfully", nil)
}
