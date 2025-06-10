package controllers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
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

// GetCategories handles category listing with search, pagination, and sorting
func GetCategories(c *gin.Context) {
	utils.LogInfo("GetCategories called")

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

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "created_at")

	utils.LogDebug("Query parameters - Page: %d, Limit: %d, Order: %s, SortBy: %s, Search: %s",
		page, limit, order, sortBy, search)

	// Validate pagination parameters
	if page < 1 {
		utils.LogError("Invalid page number: %d", page)
		utils.BadRequest(c, "Invalid page number", "Page number must be greater than 0")
		return
	}
	if limit < 1 || limit > 100 {
		utils.LogError("Invalid limit: %d", limit)
		utils.BadRequest(c, "Invalid limit", "Limit must be between 1 and 100")
		return
	}

	// Validate order parameter
	if order != "asc" && order != "desc" {
		utils.LogError("Invalid order parameter: %s", order)
		utils.BadRequest(c, "Invalid order parameter", "Order must be either 'asc' or 'desc'")
		return
	}

	// Validate sort_by parameter
	validSortFields := map[string]bool{"name": true, "created_at": true}
	if sortBy != "" && !validSortFields[sortBy] {
		utils.LogError("Invalid sort field: %s", sortBy)
		utils.BadRequest(c, "Invalid sort field", "Sort field must be either 'name' or 'created_at'")
		return
	}

	query := config.DB.Model(&models.Category{}).Where("deleted_at IS NULL")

	// Apply search with improved logging
	if search != "" {
		searchTerm := "%" + search + "%"
		utils.LogDebug("Applying search with term: %s", search)
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	} else {
		utils.LogDebug("No search term provided, returning all categories")
	}

	// Apply sorting
	switch sortBy {
	case "name":
		query = query.Order(fmt.Sprintf("name %s", order))
	default:
		query = query.Order(fmt.Sprintf("created_at %s", order))
	}
	utils.LogDebug("Applied sorting - SortBy: %s, Order: %s", sortBy, order)

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		utils.LogError("Failed to get total count: %v", err)
		utils.InternalServerError(c, "Failed to fetch categories", err.Error())
		return
	}
	utils.LogDebug("Total categories count: %d", total)

	// Apply pagination
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)
	utils.LogDebug("Applied pagination - Offset: %d, Limit: %d", offset, limit)

	var categories []models.Category
	if err := query.Find(&categories).Error; err != nil {
		utils.LogError("Failed to fetch categories: %v", err)
		utils.InternalServerError(c, "Failed to fetch categories", err.Error())
		return
	}

	// Log the results
	utils.LogDebug("Found %d categories", len(categories))

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
	utils.LogDebug("Prepared category data for response")

	// Get book count for each category
	for i, cat := range simpleCategories {
		var bookCount int64
		if err := config.DB.Model(&models.Book{}).Where("category_id = ?", cat["id"]).Count(&bookCount).Error; err != nil {
			utils.LogError("Failed to get book count for category %v: %v", cat["id"], err)
			continue
		}
		simpleCategories[i]["book_count"] = bookCount
	}
	utils.LogDebug("Updated book counts for categories")

	utils.LogInfo("Successfully retrieved %d categories", len(categories))
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

// ListCategories retrieves all active categories
func ListCategories(c *gin.Context) {
	utils.LogInfo("ListCategories called")
	var categories []models.Category
	if err := config.DB.Where("deleted_at IS NULL").Find(&categories).Error; err != nil {
		utils.LogError("Failed to fetch categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	utils.LogInfo("Successfully retrieved %d categories", len(categories))
	c.JSON(http.StatusOK, categories)
}

// ListBooksByCategory retrieves books for a specific category
func ListBooksByCategory(c *gin.Context) {
	utils.LogInfo("ListBooksByCategory called")
	categoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid category ID: %s", c.Param("id"))
		utils.BadRequest(c, "Invalid category ID", nil)
		return
	}
	utils.LogDebug("Processing category ID: %d", categoryID)

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		utils.LogError("Category not found: %v", err)
		utils.NotFound(c, "Category not found")
		return
	}
	utils.LogDebug("Found category: %s", category.Name)

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
	utils.LogDebug("Query prepared for category ID: %d, isAdmin: %v", categoryID, isAdmin)

	if err := config.DB.Raw(query, categoryID).Scan(&books).Error; err != nil {
		utils.LogError("Failed to fetch books: %v", err)
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	utils.LogInfo("Successfully retrieved %d books for category %s", len(books), category.Name)
	utils.Success(c, "Books retrieved successfully", gin.H{
		"category": gin.H{
			"id":          category.ID,
			"name":        category.Name,
			"description": category.Description,
		},
		"books": books,
	})
}
