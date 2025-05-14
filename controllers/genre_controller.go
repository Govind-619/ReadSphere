package controllers

import (
	"log"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
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

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"error": "Name and description are required",
		})
		return
	}

	// Check if genre with same name already exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ?", req.Name).First(&existingGenre).Error; err == nil {
		log.Printf("Genre with name %s already exists", req.Name)
		utils.Conflict(c, "A genre with this name already exists", nil)
		return
	}

	genre := models.Genre{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&genre).Error; err != nil {
		log.Printf("Failed to create genre: %v", err)
		utils.InternalServerError(c, "Failed to create genre", err.Error())
		return
	}

	log.Printf("Genre created successfully: %s", genre.Name)
	utils.Success(c, "Genre created successfully", gin.H{
		"genre": gin.H{
			"id":          genre.ID,
			"name":        genre.Name,
			"description": genre.Description,
		},
	})
}

// UpdateGenre handles genre updates
func UpdateGenre(c *gin.Context) {
	log.Printf("UpdateGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		log.Printf("Genre ID not provided")
		utils.BadRequest(c, "Genre ID is required", nil)
		return
	}

	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"error": "Name and description are required",
		})
		return
	}

	// Check if another genre with the same name exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ? AND id != ?", req.Name, genreID).First(&existingGenre).Error; err == nil {
		log.Printf("Another genre with name %s already exists", req.Name)
		utils.Conflict(c, "Another genre with this name already exists", nil)
		return
	}

	genre.Name = req.Name
	genre.Description = req.Description

	if err := config.DB.Save(&genre).Error; err != nil {
		log.Printf("Failed to update genre: %v", err)
		utils.InternalServerError(c, "Failed to update genre", err.Error())
		return
	}

	log.Printf("Genre updated successfully: %s", genre.Name)
	utils.Success(c, "Genre updated successfully", gin.H{
		"genre": gin.H{
			"id":          genre.ID,
			"name":        genre.Name,
			"description": genre.Description,
		},
	})
}

// DeleteGenre handles genre deletion
func DeleteGenre(c *gin.Context) {
	log.Printf("DeleteGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		log.Printf("Genre ID not provided")
		utils.BadRequest(c, "Genre ID is required", nil)
		return
	}

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}

	// Check if genre has any books
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("genre_id = ?", genreID).Count(&bookCount).Error; err != nil {
		log.Printf("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to check genre usage", err.Error())
		return
	}

	if bookCount > 0 {
		log.Printf("Cannot delete genre with %d books", bookCount)
		utils.BadRequest(c, "Cannot delete genre that has books associated with it", gin.H{
			"book_count": bookCount,
		})
		return
	}

	if err := config.DB.Delete(&genre).Error; err != nil {
		log.Printf("Failed to delete genre: %v", err)
		utils.InternalServerError(c, "Failed to delete genre", err.Error())
		return
	}

	log.Printf("Genre deleted successfully: %s", genre.Name)
	utils.Success(c, "Genre deleted successfully", nil)
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
		utils.InternalServerError(c, "Failed to count genres", err.Error())
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
		utils.InternalServerError(c, "Failed to fetch genres", err.Error())
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

	utils.Success(c, "Genres retrieved successfully", gin.H{
		"genres": simpleGenres,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
		"sort": gin.H{
			"by":    sortBy,
			"order": order,
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
		utils.BadRequest(c, "Genre ID is required", nil)
		return
	}

	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}

	// Get book count for this genre
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("genre_id = ?", genreID).Count(&bookCount).Error; err != nil {
		log.Printf("Failed to get book count: %v", err)
		utils.InternalServerError(c, "Failed to get genre details", err.Error())
		return
	}

	log.Printf("Found genre: %s", genre.Name)
	utils.Success(c, "Genre details retrieved successfully", gin.H{
		"genre": gin.H{
			"id":          genre.ID,
			"name":        genre.Name,
			"description": genre.Description,
			"book_count":  bookCount,
		},
	})
}

// ListBooksByGenre retrieves books for a specific genre
func ListBooksByGenre(c *gin.Context) {
	log.Printf("ListBooksByGenre called")

	genreID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		log.Printf("Invalid genre ID: %v", err)
		utils.BadRequest(c, "Invalid genre ID", nil)
		return
	}

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}

	// Fetch books for the genre with only essential fields
	type BookResponse struct {
		ID          uint    `json:"id"`
		Name        string  `json:"name"`
		Author      string  `json:"author"`
		Price       float64 `json:"price"`
		ImageURL    string  `json:"image_url"`
		Description string  `json:"description"`
		IsActive    bool    `json:"is_active,omitempty"`
		Stock       int     `json:"stock,omitempty"`
	}

	var books []BookResponse
	query := `
		SELECT 
			id, name, author, price, image_url, description, is_active, stock
		FROM books 
		WHERE genre_id = ? AND deleted_at IS NULL
	`

	_, isAdmin := c.Get("admin")
	if !isAdmin {
		query += " AND is_active = true"
	}

	if err := config.DB.Raw(query, genreID).Scan(&books).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	log.Printf("Found %d books for genre %s", len(books), genre.Name)
	utils.Success(c, "Books retrieved successfully", gin.H{
		"genre": gin.H{
			"id":          genre.ID,
			"name":        genre.Name,
			"description": genre.Description,
		},
		"books": books,
	})
}
