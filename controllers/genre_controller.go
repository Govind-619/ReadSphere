package controllers

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

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

	var genres []models.Genre
	if err := config.DB.Find(&genres).Error; err != nil {
		log.Printf("Failed to fetch genres: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch genres"})
		return
	}

	log.Printf("Found %d genres", len(genres))
	c.JSON(http.StatusOK, gin.H{
		"genres": genres,
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

	// Get books for the genre using a raw SQL query to handle the text[] column properly
	var books []models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, stock, category_id, 
			image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages
		FROM books 
		WHERE genre_id = ? AND is_active = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, genreID, true).Scan(&books).Error; err != nil {
		log.Printf("Failed to fetch books: %v", err)
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
			books[i].Images = strings.Join(images, ",")
		}
	}

	log.Printf("Found %d books for genre %s", len(books), genre.Name)
	c.JSON(http.StatusOK, gin.H{
		"genre": genre,
		"books": books,
	})
}
