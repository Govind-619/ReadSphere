package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/utils"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// CategoryListRequest represents the request parameters for category listing
type CategoryListRequest struct {
	Page   int    `form:"page" binding:"min=1"`
	Limit  int    `form:"limit" binding:"min=1,max=100"`
	Search string `form:"search"`
	SortBy string `form:"sort_by"`
	Order  string `form:"order" binding:"oneof=asc desc"`
}

// CategoryRequest represents the category creation/update request
type CategoryRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"required,min=10,max=500"`
}

// CategoryBlockRequest represents the block/unblock request
type CategoryBlockRequest struct {
	Blocked bool `json:"blocked"`
}

// GetCategories handles category listing with search, pagination, and sorting
func GetCategories(c *gin.Context) {
	log.Printf("GetCategories called")

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

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "created_at")

	// Validate pagination parameters
	if page < 1 {
		utils.BadRequest(c, "Invalid page number", "Page number must be greater than 0")
		return
	}
	if limit < 1 || limit > 100 {
		utils.BadRequest(c, "Invalid limit", "Limit must be between 1 and 100")
		return
	}

	// Validate order parameter
	if order != "asc" && order != "desc" {
		utils.BadRequest(c, "Invalid order parameter", "Order must be either 'asc' or 'desc'")
		return
	}

	// Validate sort_by parameter
	validSortFields := map[string]bool{"name": true, "created_at": true}
	if sortBy != "" && !validSortFields[sortBy] {
		utils.BadRequest(c, "Invalid sort field", "Sort field must be either 'name' or 'created_at'")
		return
	}

	query := config.DB.Model(&models.Category{}).Where("deleted_at IS NULL")

	// Apply search with improved logging
	if search != "" {
		searchTerm := "%" + search + "%"
		log.Printf("Applying search with term: %s", search)
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	} else {
		log.Printf("No search term provided, returning all categories")
	}

	// Apply sorting
	switch sortBy {
	case "name":
		query = query.Order(fmt.Sprintf("name %s", order))
	default:
		query = query.Order(fmt.Sprintf("created_at %s", order))
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		log.Printf("Failed to get total count: %v", err)
		utils.InternalServerError(c, "Failed to fetch categories", err.Error())
		return
	}

	// Apply pagination
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// Add debug logging
	log.Printf("Query parameters - Page: %d, Limit: %d, Order: %s, SortBy: %s, Search: %s",
		page, limit, order, sortBy, search)

	var categories []models.Category
	if err := query.Find(&categories).Error; err != nil {
		log.Printf("Failed to fetch categories: %v", err)
		utils.InternalServerError(c, "Failed to fetch categories", err.Error())
		return
	}

	// Log the results
	log.Printf("Found %d categories", len(categories))

	// Return standardized response
	var simpleCategories []gin.H
	for _, cat := range categories {
		simpleCategories = append(simpleCategories, gin.H{
			"id":          cat.ID,
			"name":        cat.Name,
			"description": cat.Description,
			"blocked":     cat.Blocked,
			"created_at":  cat.CreatedAt,
			"updated_at":  cat.UpdatedAt,
			"book_count":  0, // This will be updated in the next iteration
		})
	}

	// Get book count for each category
	for i, cat := range simpleCategories {
		var bookCount int64
		if err := config.DB.Model(&models.Book{}).Where("category_id = ?", cat["id"]).Count(&bookCount).Error; err != nil {
			log.Printf("Failed to get book count for category %v: %v", cat["id"], err)
			continue
		}
		simpleCategories[i]["book_count"] = bookCount
	}

	utils.Success(c, "Categories retrieved successfully", gin.H{
		"categories": simpleCategories,
		"pagination": gin.H{
			"total":        total,
			"current_page": page,
			"per_page":     limit,
			"total_pages":  (total + int64(limit) - 1) / int64(limit),
			"has_more":     (int64(page)*int64(limit) < total),
		},
		"filters": gin.H{
			"search": search,
			"sort":   sortBy,
			"order":  order,
		},
	})
}

// CreateCategory handles category creation
func CreateCategory(c *gin.Context) {
	log.Printf("CreateCategory called")

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

	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	// Check if category with same name already exists
	var existingCategory models.Category
	if err := config.DB.Where("name = ?", req.Name).First(&existingCategory).Error; err == nil {
		log.Printf("Category with name %s already exists", req.Name)
		utils.Conflict(c, "A category with this name already exists", nil)
		return
	}

	category := models.Category{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&category).Error; err != nil {
		log.Printf("Failed to create category: %v", err)
		utils.InternalServerError(c, "Failed to create category", err.Error())
		return
	}

	log.Printf("Category created successfully: %s", category.Name)
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
	log.Printf("UpdateCategory called")

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

	// Validate and parse category ID
	categoryID := c.Param("id")
	if categoryID == "" {
		log.Printf("Category ID not provided")
		utils.BadRequest(c, "Category ID is required", nil)
		return
	}

	id, err := strconv.ParseUint(categoryID, 10, 32)
	if err != nil {
		log.Printf("Invalid category ID format: %v", err)
		utils.BadRequest(c, "Invalid category ID format", "Category ID must be a valid number")
		return
	}

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, id).Error; err != nil {
		log.Printf("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}

	// Parse and validate request body
	var req CategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"name":        "Name is required and must be between 2 and 100 characters",
			"description": "Description is required and must be between 10 and 500 characters",
		})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process update", nil)
		return
	}

	// Check for duplicate name excluding current category
	var existingCategory models.Category
	if err := tx.Where("name ILIKE ? AND id != ?", req.Name, id).First(&existingCategory).Error; err == nil {
		tx.Rollback()
		log.Printf("Duplicate category name found: %s", req.Name)
		utils.Conflict(c, "Category name already exists", "Please choose a different name")
		return
	}

	// Update category fields
	updates := map[string]interface{}{
		"name":        strings.TrimSpace(req.Name),
		"description": strings.TrimSpace(req.Description),
		"updated_at":  time.Now(),
	}

	// Apply updates
	if err := tx.Model(&category).Updates(updates).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update category: %v", err)
		utils.InternalServerError(c, "Failed to update category", err.Error())
		return
	}

	// Get book count
	var bookCount int64
	if err := tx.Model(&models.Book{}).Where("category_id = ?", id).Count(&bookCount).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to get book count: %v", err)
		utils.InternalServerError(c, "Failed to get category details", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to save changes", err.Error())
		return
	}

	log.Printf("Category updated successfully: %s", category.Name)
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
	log.Printf("DeleteCategory called")

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

	categoryID := c.Param("id")
	if categoryID == "" {
		log.Printf("Category ID not provided")
		utils.BadRequest(c, "Category ID is required", nil)
		return
	}

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		log.Printf("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}

	// Check if category has any books
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("category_id = ?", categoryID).Count(&bookCount).Error; err != nil {
		log.Printf("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to check category usage", err.Error())
		return
	}

	if bookCount > 0 {
		log.Printf("Cannot delete category with %d books", bookCount)
		utils.BadRequest(c, "Cannot delete category that has books associated with it", gin.H{
			"book_count": bookCount,
		})
		return
	}

	if err := config.DB.Delete(&category).Error; err != nil {
		log.Printf("Failed to delete category: %v", err)
		utils.InternalServerError(c, "Failed to delete category", err.Error())
		return
	}

	log.Printf("Category deleted successfully: %s", category.Name)
	utils.Success(c, "Category deleted successfully", nil)
}

// ListCategories retrieves all active categories
func ListCategories(c *gin.Context) {
	var categories []models.Category
	if err := config.DB.Where("deleted_at IS NULL").Find(&categories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

// ListBooksByCategory retrieves books for a specific category
func ListBooksByCategory(c *gin.Context) {
	categoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.BadRequest(c, "Invalid category ID", nil)
		return
	}

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		utils.NotFound(c, "Category not found")
		return
	}

	// Fetch books for the category with only essential fields
	type BookResponse struct {
		ID          uint    `json:"id"`
		Name        string  `json:"name"`
		Author      string  `json:"author"`
		Price       float64 `json:"price"`
		Stock       int     `json:"stock"`
		ImageURL    string  `json:"image_url"`
		Description string  `json:"description"`
		IsActive    bool    `json:"is_active"`
	}

	var books []BookResponse
	query := `
		SELECT 
			id, name, author, price, stock, image_url, description, is_active
		FROM books 
		WHERE category_id = ? AND deleted_at IS NULL
	`

	_, isAdmin := c.Get("admin")
	if !isAdmin {
		query += " AND is_active = true"
	}

	if err := config.DB.Raw(query, categoryID).Scan(&books).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	utils.Success(c, "Books retrieved successfully", gin.H{
		"category": gin.H{
			"id":          category.ID,
			"name":        category.Name,
			"description": category.Description,
		},
		"books": books,
	})
}

// CreateDefaultCategory creates a default category if none exists
func CreateDefaultCategory() error {
	var count int64
	if err := config.DB.Model(&models.Category{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		log.Printf("No categories found, creating default category")
		defaultCategory := models.Category{
			Name:        "General",
			Description: "Default category for books",
		}
		return config.DB.Create(&defaultCategory).Error
	}

	return nil
}

// EnsureCategoryExists checks if a category exists by ID and creates it if it doesn't
func EnsureCategoryExists(categoryID uint) error {
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		log.Printf("Category %d not found, creating it", categoryID)
		category = models.Category{
			Name:        fmt.Sprintf("Category %d", categoryID),
			Description: fmt.Sprintf("Description for category %d", categoryID),
		}
		return config.DB.Create(&category).Error
	}
	return nil
}

// EnsureGenreExists checks if a genre exists by ID and creates it if it doesn't
func EnsureGenreExists(genreID uint) error {
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre %d not found, creating it", genreID)
		genre = models.Genre{
			Name:        fmt.Sprintf("Genre %d", genreID),
			Description: fmt.Sprintf("Description for genre %d", genreID),
		}
		return config.DB.Create(&genre).Error
	}
	return nil
}

// ToggleCategoryBlock handles blocking/unblocking of categories
func ToggleCategoryBlock(c *gin.Context) {
	log.Printf("ToggleCategoryBlock called")

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

	// Validate and parse category ID
	categoryID := c.Param("id")
	if categoryID == "" {
		log.Printf("Category ID not provided")
		utils.BadRequest(c, "Category ID is required", nil)
		return
	}

	id, err := strconv.ParseUint(categoryID, 10, 32)
	if err != nil {
		log.Printf("Invalid category ID format: %v", err)
		utils.BadRequest(c, "Invalid category ID format", "Category ID must be a valid number")
		return
	}

	// Parse request body
	var req CategoryBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", "Invalid request format")
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process request", nil)
		return
	}

	// Get category
	var category models.Category
	if err := tx.First(&category, id).Error; err != nil {
		tx.Rollback()
		log.Printf("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}

	// Update blocked status
	if err := tx.Model(&category).Updates(map[string]interface{}{
		"blocked":    req.Blocked,
		"updated_at": time.Now(),
	}).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to update category block status: %v", err)
		utils.InternalServerError(c, "Failed to update category", err.Error())
		return
	}

	// If blocking, also block all books in this category
	if req.Blocked {
		if err := tx.Model(&models.Book{}).Where("category_id = ?", id).Updates(map[string]interface{}{
			"blocked":    true,
			"updated_at": time.Now(),
		}).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to block category books: %v", err)
			utils.InternalServerError(c, "Failed to block category books", err.Error())
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to save changes", err.Error())
		return
	}

	action := "blocked"
	if !req.Blocked {
		action = "unblocked"
	}

	log.Printf("Category %s successfully: %s", action, category.Name)
	utils.Success(c, fmt.Sprintf("Category %s successfully", action), gin.H{
		"category": gin.H{
			"id":      category.ID,
			"blocked": req.Blocked,
		},
	})
}
