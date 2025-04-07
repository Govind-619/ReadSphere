package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

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

// GetCategories handles category listing with search, pagination, and sorting
func GetCategories(c *gin.Context) {
	log.Printf("GetCategories called")

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	sortBy := c.Query("sort_by")

	// Create request struct with default values
	req := CategoryListRequest{
		Page:   page,
		Limit:  limit,
		Order:  order,
		Search: search,
		SortBy: sortBy,
	}

	// Validate the request
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := config.DB.Model(&models.Category{})

	// Apply search with improved logging
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		log.Printf("Applying search with term: %s", req.Search)
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	} else {
		log.Printf("No search term provided, returning all categories")
	}

	// Apply sorting
	switch req.SortBy {
	case "name":
		query = query.Order(fmt.Sprintf("name %s", req.Order))
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

	var categories []models.Category
	if err := query.Find(&categories).Error; err != nil {
		log.Printf("Failed to fetch categories: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch categories"})
		return
	}

	// Log the results
	log.Printf("Found %d categories", len(categories))
	for i, category := range categories {
		log.Printf("Category %d: ID=%d, Name=%s, CreatedAt=%v", i+1, category.ID, category.Name, category.CreatedAt)
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": categories,
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

// CreateCategory handles category creation
func CreateCategory(c *gin.Context) {
	var category models.Category
	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if err := config.DB.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}

	c.JSON(http.StatusCreated, category)
}

// UpdateCategory handles category updates
func UpdateCategory(c *gin.Context) {
	categoryID := c.Param("id")
	var category models.Category

	if err := config.DB.First(&category, categoryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	if err := c.ShouldBindJSON(&category); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	if err := config.DB.Save(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update category"})
		return
	}

	c.JSON(http.StatusOK, category)
}

// DeleteCategory handles soft deletion of a category
func DeleteCategory(c *gin.Context) {
	categoryID := c.Param("id")

	if err := config.DB.Delete(&models.Category{}, categoryID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete category"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	// Check if category exists
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}

	// Get books for the category using a raw SQL query to handle the text[] column properly
	var books []models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, stock, category_id, 
			image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages
		FROM books 
		WHERE category_id = ? AND is_active = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, categoryID, true).Scan(&books).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books"})
		return
	}

	// Now fetch the images separately for each book
	for i := range books {
		var images []string
		if err := config.DB.Raw("SELECT images FROM books WHERE id = ?", books[i].ID).Scan(&images).Error; err != nil {
			log.Printf("Failed to fetch images for book %d: %v", books[i].ID, err)
			// Continue anyway, as we have the book data
		} else {
			books[i].Images = images
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"category": category,
		"books":    books,
		"note":     "Prices are in Indian Rupees (INR)",
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
