package controllers

import (
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

// GetGenres handles listing all genres
func GetGenres(c *gin.Context) {
	utils.LogInfo("GetGenres called")

	// Set default values for query parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	order := c.DefaultQuery("order", "desc")
	search := c.Query("search")
	sortBy := c.DefaultQuery("sort_by", "created_at")

	utils.LogDebug("Query parameters - Page: %d, Limit: %d, Order: %s, Search: %s, SortBy: %s",
		page, limit, order, search, sortBy)

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
		utils.LogDebug("Applied search filter with term: %s", search)
	}

	total := int64(0)
	if err := query.Count(&total).Error; err != nil {
		utils.LogError("Failed to count genres: %v", err)
		utils.InternalServerError(c, "Failed to count genres", err.Error())
		return
	}
	utils.LogDebug("Total genres found: %d", total)

	if sortBy == "" {
		sortBy = "created_at"
	}
	query = query.Order(sortBy + " " + order)
	utils.LogDebug("Applied sorting: %s %s", sortBy, order)

	// Pagination
	offset := (page - 1) * limit
	query = query.Limit(limit).Offset(offset)
	utils.LogDebug("Applied pagination - Offset: %d, Limit: %d", offset, limit)

	var genres []models.Genre
	if err := query.Find(&genres).Error; err != nil {
		utils.LogError("Failed to fetch genres: %v", err)
		utils.InternalServerError(c, "Failed to fetch genres", err.Error())
		return
	}

	utils.LogDebug("Retrieved %d genres", len(genres))
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
	utils.LogDebug("Converted genres to simple format")

	utils.LogInfo("Successfully retrieved %d genres", len(simpleGenres))
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
