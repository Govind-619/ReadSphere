package controllers

import (
	"log"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

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
