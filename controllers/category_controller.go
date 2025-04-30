package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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

	query := config.DB.Model(&models.Category{}).Where("deleted_at IS NULL")

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

	// Return only id, name, description, blocked fields
	var simpleCategories []gin.H
	for _, cat := range categories {
		simpleCategories = append(simpleCategories, gin.H{
			"id":          cat.ID,
			"name":        cat.Name,
			"description": cat.Description,
			"blocked":     cat.Blocked,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": simpleCategories,
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

	// Check for duplicate category name (case-insensitive)
	var existing models.Category
	if err := config.DB.Where("LOWER(name) = ? AND deleted_at IS NULL", strings.ToLower(category.Name)).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Category with this name already exists"})
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

	// Fetch books for the category and preload related images from BookImage table
	var books []models.Book
	if err := config.DB.Preload("BookImages").Where("category_id = ? AND is_active = ? AND deleted_at IS NULL", categoryID, true).Find(&books).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books"})
		return
	}

	// Define a struct for admin and non-admin book responses
	type AdminBook struct {
		ID                 uint               `json:"id"`
		Name               string             `json:"name"`
		Description        string             `json:"description"`
		Price              float64            `json:"price"`
		OriginalPrice      float64            `json:"original_price"`
		DiscountPercentage int                `json:"discount_percentage"`
		Stock              int                `json:"stock"`
		CategoryID         uint               `json:"category_id"`
		GenreID            uint               `json:"genre_id"`
		ImageURL           string             `json:"image_url"`
		Images             []models.BookImage `json:"images"`
		IsActive           bool               `json:"is_active"`
		IsFeatured         bool               `json:"is_featured"`
		Views              int                `json:"views"`
		AverageRating      float64            `json:"average_rating"`
		TotalReviews       int                `json:"total_reviews"`
		Author             string             `json:"author"`
		Publisher          string             `json:"publisher"`
		ISBN               string             `json:"isbn"`
		PublicationYear    int                `json:"publication_year"`
		Pages              int                `json:"pages"`
		Language           string             `json:"language"`
		Format             string             `json:"format"`
		Blocked            bool               `json:"blocked"`
	}
	type PublicBook struct {
		ID                 uint               `json:"id"`
		Name               string             `json:"name"`
		Description        string             `json:"description"`
		Price              float64            `json:"price"`
		OriginalPrice      float64            `json:"original_price"`
		DiscountPercentage int                `json:"discount_percentage"`
		CategoryID         uint               `json:"category_id"`
		GenreID            uint               `json:"genre_id"`
		ImageURL           string             `json:"image_url"`
		Images             []models.BookImage `json:"images"`
		IsActive           bool               `json:"is_active"`
		IsFeatured         bool               `json:"is_featured"`
		Views              int                `json:"views"`
		AverageRating      float64            `json:"average_rating"`
		TotalReviews       int                `json:"total_reviews"`
		Author             string             `json:"author"`
		Publisher          string             `json:"publisher"`
		ISBN               string             `json:"isbn"`
		PublicationYear    int                `json:"publication_year"`
		Pages              int                `json:"pages"`
		Language           string             `json:"language"`
		Format             string             `json:"format"`
	}

	_, isAdmin := c.Get("admin")
	if isAdmin {
		var adminBooks []AdminBook
		for _, b := range books {
			adminBooks = append(adminBooks, AdminBook{
				ID:                 b.ID,
				Name:               b.Name,
				Description:        b.Description,
				Price:              b.Price,
				OriginalPrice:      b.OriginalPrice,
				DiscountPercentage: b.DiscountPercentage,
				Stock:              b.Stock,
				CategoryID:         b.CategoryID,
				GenreID:            b.GenreID,
				ImageURL:           b.ImageURL,
				Images:             b.BookImages,
				IsActive:           b.IsActive,
				IsFeatured:         b.IsFeatured,
				Views:              b.Views,
				AverageRating:      b.AverageRating,
				TotalReviews:       b.TotalReviews,
				Author:             b.Author,
				Publisher:          b.Publisher,
				ISBN:               b.ISBN,
				PublicationYear:    b.PublicationYear,
				Pages:              b.Pages,
				Language:           b.Language,
				Format:             b.Format,
				Blocked:            b.Blocked,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"category": category,
			"books":    adminBooks,
			"note":     "Prices are in Indian Rupees (INR)",
		})
	} else {
		var publicBooks []PublicBook
		for _, b := range books {
			publicBooks = append(publicBooks, PublicBook{
				ID:                 b.ID,
				Name:               b.Name,
				Description:        b.Description,
				Price:              b.Price,
				OriginalPrice:      b.OriginalPrice,
				DiscountPercentage: b.DiscountPercentage,
				CategoryID:         b.CategoryID,
				GenreID:            b.GenreID,
				ImageURL:           b.ImageURL,
				Images:             b.BookImages,
				IsActive:           b.IsActive,
				IsFeatured:         b.IsFeatured,
				Views:              b.Views,
				AverageRating:      b.AverageRating,
				TotalReviews:       b.TotalReviews,
				Author:             b.Author,
				Publisher:          b.Publisher,
				ISBN:               b.ISBN,
				PublicationYear:    b.PublicationYear,
				Pages:              b.Pages,
				Language:           b.Language,
				Format:             b.Format,
			})
		}
		c.JSON(http.StatusOK, gin.H{
			"category": category,
			"books":    publicBooks,
		})
	}
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
