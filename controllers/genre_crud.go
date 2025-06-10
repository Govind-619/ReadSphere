package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GenreRequest represents the genre creation/update request
type GenreRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
}

// CreateGenre handles genre creation
func CreateGenre(c *gin.Context) {
	utils.LogInfo("CreateGenre called")

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

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"error": "Name and description are required",
		})
		return
	}
	utils.LogDebug("Received genre creation request - Name: %s", req.Name)

	// Check if genre with same name already exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ?", req.Name).First(&existingGenre).Error; err == nil {
		utils.LogError("Genre with name %s already exists", req.Name)
		utils.Conflict(c, "A genre with this name already exists", nil)
		return
	}
	utils.LogDebug("No existing genre found with name: %s", req.Name)

	genre := models.Genre{
		Name:        req.Name,
		Description: req.Description,
	}

	if err := config.DB.Create(&genre).Error; err != nil {
		utils.LogError("Failed to create genre: %v", err)
		utils.InternalServerError(c, "Failed to create genre", err.Error())
		return
	}

	utils.LogInfo("Genre created successfully: %s", genre.Name)
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
	utils.LogInfo("UpdateGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		utils.LogError("Genre ID not provided")
		utils.BadRequest(c, "Genre ID is required", nil)
		return
	}
	utils.LogDebug("Updating genre with ID: %s", genreID)

	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		utils.LogError("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}
	utils.LogDebug("Found genre to update: %s", genre.Name)

	var req GenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", gin.H{
			"error": "Name and description are required",
		})
		return
	}
	utils.LogDebug("Received genre update request - Name: %s", req.Name)

	// Check if another genre with the same name exists
	var existingGenre models.Genre
	if err := config.DB.Where("name = ? AND id != ?", req.Name, genreID).First(&existingGenre).Error; err == nil {
		utils.LogError("Another genre with name %s already exists", req.Name)
		utils.Conflict(c, "Another genre with this name already exists", nil)
		return
	}
	utils.LogDebug("No name conflict found for genre update")

	genre.Name = req.Name
	genre.Description = req.Description

	if err := config.DB.Save(&genre).Error; err != nil {
		utils.LogError("Failed to update genre: %v", err)
		utils.InternalServerError(c, "Failed to update genre", err.Error())
		return
	}

	utils.LogInfo("Genre updated successfully: %s", genre.Name)
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
	utils.LogInfo("DeleteGenre called")

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

	genreID := c.Param("id")
	if genreID == "" {
		utils.LogError("Genre ID not provided")
		utils.BadRequest(c, "Genre ID is required", nil)
		return
	}
	utils.LogDebug("Attempting to delete genre with ID: %s", genreID)

	// Check if genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		utils.LogError("Genre not found: %v", err)
		utils.NotFound(c, "Genre not found")
		return
	}
	utils.LogDebug("Found genre to delete: %s", genre.Name)

	// Check if genre has any books
	var bookCount int64
	if err := config.DB.Model(&models.Book{}).Where("genre_id = ?", genreID).Count(&bookCount).Error; err != nil {
		utils.LogError("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to check genre usage", err.Error())
		return
	}
	utils.LogDebug("Found %d books associated with genre", bookCount)

	if bookCount > 0 {
		utils.LogError("Cannot delete genre with %d books", bookCount)
		utils.BadRequest(c, "Cannot delete genre that has books associated with it", gin.H{
			"book_count": bookCount,
		})
		return
	}

	if err := config.DB.Delete(&genre).Error; err != nil {
		utils.LogError("Failed to delete genre: %v", err)
		utils.InternalServerError(c, "Failed to delete genre", err.Error())
		return
	}

	utils.LogInfo("Genre deleted successfully: %s", genre.Name)
	utils.Success(c, "Genre deleted successfully", nil)
}
