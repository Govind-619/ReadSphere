package controllers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// ListRequest represents the request parameters for category listing
type GenreListRequest struct {
	Page   int    `form:"page" binding:"min=1"`
	Limit  int    `form:"limit" binding:"min=1,max=100"`
	Search string `form:"search"`
	SortBy string `form:"sort_by"`
	Order  string `form:"order" binding:"oneof=asc desc"`
}

// GenreRequest represents the genre creation/update request
type GenreRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

// CreateGenre handles genre creation
func CreateGenre(c *gin.Context) {
	log.Printf("CreateGenre called")

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

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if genre with same name already exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ?", req.Name).First(&existingGenre).Error; err == nil {
		log.Printf("Genre with name %s already exists", req.Name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "A genre with this name already exists"})
		return
	}

	genre := models.Genre{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&genre).Error; err != nil {
		log.Printf("Failed to create genre: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create genre"})
		return
	}

	log.Printf("Genre created successfully: %s", genre.Name)
	c.JSON(http.StatusCreated, gin.H{
		"message": "Genre created successfully",
		"genre":   genre,
	})
}

// UpdateGenre handles genre updates
func UpdateGenre(c *gin.Context) {
	log.Printf("UpdateGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		log.Printf("Genre ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Genre ID is required"})
		return
	}

	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if another genre with the same name exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ? AND id != ?", req.Name, genreID).First(&existingGenre).Error; err == nil {
		log.Printf("Another genre with name %s already exists", req.Name)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Another genre with this name already exists"})
		return
	}

	genre.Name = req.Name
	genre.Description = req.Description

	if err := config.DB.Save(&genre).Error; err != nil {
		log.Printf("Failed to update genre: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update genre"})
		return
	}

	log.Printf("Genre updated successfully: %s", genre.Name)
	c.JSON(http.StatusOK, gin.H{
		"message": "Genre updated successfully",
		"genre":   genre,
	})
}

// DeleteGenre handles genre deletion
func DeleteGenre(c *gin.Context) {
	log.Printf("DeleteGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		log.Printf("Genre ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Genre ID is required"})
		return
	}

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	// Check if genre has any books
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("genre_id = ?", genreID).Count(&bookCount).Error; err != nil {
		log.Printf("Failed to count books: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check genre usage"})
		return
	}

	if bookCount > 0 {
		log.Printf("Cannot delete genre with %d books", bookCount)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete genre that has books associated with it"})
		return
	}

	if err := config.DB.Delete(&genre).Error; err != nil {
		log.Printf("Failed to delete genre: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete genre"})
		return
	}

	log.Printf("Genre deleted successfully: %s", genre.Name)
	c.JSON(http.StatusOK, gin.H{
		"message": "Genre deleted successfully",
	})
}

// GetGenres handles listing all genres
func GetGenres(c *gin.Context) {
	log.Printf("GetGenres called")

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "created_at")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	// Build query
	query := config.DB.Model(&models.Genre{})
	if search != "" {
		searchTerm := "%" + search + "%"
		query = query.Where("name ILIKE ? OR description ILIKE ?", searchTerm, searchTerm)
	}

	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		log.Printf("Failed to count genres: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count genres"})
		return
	}

	if sortBy == "" {
		sortBy = "created_at"
	}
	query = query.Order(sortBy + " " + order)

	// Pagination
	offset := (page - 1) * limit
	query = query.Limit(limit).Offset(offset)

	var genres []models.Genre
	if err := query.Find(&genres).Error; err != nil {
		log.Printf("Failed to fetch genres: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch genres"})
		return
	}

	log.Printf("Found %d genres", len(genres))
	type SimpleGenre struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	var simpleGenres []SimpleGenre
	for _, genre := range genres {
		simpleGenres = append(simpleGenres, SimpleGenre{
			ID:          genre.ID,
			Name:        genre.Name,
			Description: genre.Description,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"genres": simpleGenres,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
		"search": gin.H{
			"term": search,
		},
	})
}

// GetGenreDetails handles fetching a single genre's details
func GetGenreDetails(c *gin.Context) {
	log.Printf("GetGenreDetails called")

	genreID := c.Param("id")
	if genreID == "" {
		log.Printf("Genre ID not provided")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Genre ID is required"})
		return
	}

	var genre models.Genre
	if err := config.DB.Preload("Books").First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	log.Printf("Found genre: %s", genre.Name)
	c.JSON(http.StatusOK, gin.H{
		"genre": genre,
	})
}

// ListBooksByGenre retrieves books for a specific genre
func ListBooksByGenre(c *gin.Context) {
	log.Printf("ListBooksByGenre called")

	genreID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("Invalid genre ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid genre ID"})
		return
	}

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Genre not found"})
		return
	}

	// Fetch books for the genre and preload related images from BookImage table
	var books []models.Book
	if err := config.DB.Preload("BookImages").Where("genre_id = ? AND is_active = ? AND deleted_at IS NULL", genreID, true).Find(&books).Error; err != nil {
		log.Printf("Failed to fetch books: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books"})
		return
	}

	log.Printf("Found %d books for genre %s", len(books), genre.Name)

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
			"genre": genre,
			"books": adminBooks,
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
			"genre": genre,
			"books": publicBooks,
		})
	}
}
