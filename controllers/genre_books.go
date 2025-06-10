package controllers

import (
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ListBooksByGenre retrieves books for a specific genre
func ListBooksByGenre(c *gin.Context) {
	utils.LogInfo("ListBooksByGenre called")

	genreID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		utils.LogError("Invalid genre ID: %v", err)
		utils.BadRequest(c, "Invalid genre ID", nil)
		return
	}
	utils.LogDebug("Processing genre ID: %d", genreID)

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		utils.LogError("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}
	utils.LogDebug("Found genre: %s", genre.Name)

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
		utils.LogDebug("Applied active books filter for non-admin user")
	}

	if err := config.DB.Raw(query, genreID).Scan(&books).Error; err != nil {
		utils.LogError("Failed to fetch books: %v", err)
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	utils.LogInfo("Found %d books for genre %s", len(books), genre.Name)
	utils.Success(c, "Books retrieved successfully", gin.H{
		"genre": gin.H{
			"id":          genre.ID,
			"name":        genre.Name,
			"description": genre.Description,
		},
		"books": books,
	})
}
