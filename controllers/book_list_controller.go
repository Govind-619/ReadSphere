package controllers

import (
	"fmt"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// BookListRequest represents the request parameters for listing books
type BookListRequest struct {
	Page         int     `form:"page" binding:"min=1"`
	Limit        int     `form:"limit" binding:"min=1,max=100"`
	Order        string  `form:"order" binding:"oneof=asc desc"`
	Search       string  `form:"search"`
	SortBy       string  `form:"sort_by" binding:"oneof=name price created_at views average_rating"`
	CategoryID   uint    `form:"category_id"`
	GenreID      uint    `form:"genre_id"`
	MinPrice     float64 `form:"min_price"`
	MaxPrice     float64 `form:"max_price"`
	IsNewArrival bool    `form:"new_arrival"`
	IsFeatured   bool    `form:"featured"`
}

// BookListItem represents a minimal book item for list view
type BookListItem struct {
	ID       uint    `json:"id"`
	Name     string  `json:"name"`
	Author   string  `json:"author"`
	Price    float64 `json:"price"`
	ImageURL string  `json:"image_url"`
	IsActive bool    `json:"is_active"`
	Stock    int     `json:"stock"`
}

// BookListResponse represents the ordered response for book listing
type BookListResponse struct {
	Books            []BookListItem `json:"books"`
	Pagination       gin.H          `json:"pagination"`
	Sort             gin.H          `json:"sort"`
	Filters          gin.H          `json:"filters"`
	AvailableFilters gin.H          `json:"available_filters"`
}

// GetBooks handles listing books with search, pagination, and sorting
func GetBooks(c *gin.Context) {
	utils.LogInfo("GetBooks called with query params: %v", c.Request.URL.Query())

	var req BookListRequest

	// Set default values for query parameters
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "10"))
	req.SortBy = c.DefaultQuery("sort_by", "created_at")
	req.Order = c.DefaultQuery("order", "desc")
	req.Search = c.Query("search")
	if categoryID, err := strconv.ParseUint(c.Query("category_id"), 10, 32); err == nil {
		req.CategoryID = uint(categoryID)
	}
	if genreID, err := strconv.ParseUint(c.Query("genre_id"), 10, 32); err == nil {
		req.GenreID = uint(genreID)
	}
	req.IsNewArrival = c.Query("new_arrival") == "true"
	req.IsFeatured = c.Query("featured") == "true"
	minPrice := c.Query("min_price")
	maxPrice := c.Query("max_price")

	// Set defaults
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.Order == "" {
		req.Order = "desc"
	}

	utils.LogInfo("Processed request parameters - Page: %d, Limit: %d, SortBy: %s, Order: %s",
		req.Page, req.Limit, req.SortBy, req.Order)

	// Parse price range if provided
	if minPrice != "" {
		if price, err := strconv.ParseFloat(minPrice, 64); err == nil {
			req.MinPrice = price
		}
	}
	if maxPrice != "" {
		if price, err := strconv.ParseFloat(maxPrice, 64); err == nil {
			req.MaxPrice = price
		}
	}

	// Build the base query with only essential fields
	query := `
		SELECT 
			books.id, books.name, books.author, books.price, books.image_url, books.is_active, books.stock
		FROM books
		JOIN categories ON books.category_id = categories.id
		WHERE books.deleted_at IS NULL AND categories.deleted_at IS NULL
	`

	// Check if admin is in context - if not, only show active books
	_, isAdmin := c.Get("admin")
	if !isAdmin {
		query += " AND is_active = true"
		utils.LogInfo("Non-admin request - filtering active books only")
	}

	// Add category filter if provided
	if req.CategoryID > 0 {
		utils.LogInfo("Filtering by category_id: %d", req.CategoryID)
		query += fmt.Sprintf(" AND category_id = %d", req.CategoryID)
	}

	// Add genre filter if provided
	if req.GenreID > 0 {
		utils.LogInfo("Filtering by genre_id: %d", req.GenreID)
		query += fmt.Sprintf(" AND genre_id = %d", req.GenreID)
	}

	// Add price range filters if provided
	if req.MinPrice > 0 {
		utils.LogInfo("Filtering by min_price: %f", req.MinPrice)
		query += fmt.Sprintf(" AND price >= %f", req.MinPrice)
	}
	if req.MaxPrice > 0 {
		utils.LogInfo("Filtering by max_price: %f", req.MaxPrice)
		query += fmt.Sprintf(" AND price <= %f", req.MaxPrice)
	}

	// Add search filter if provided
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		utils.LogInfo("Filtering by search term: %s", req.Search)
		query += fmt.Sprintf(" AND (books.name ILIKE '%s' OR books.author ILIKE '%s')",
			searchTerm, searchTerm)
	}

	// Add new arrival filter if requested
	if req.IsNewArrival {
		utils.LogInfo("Filtering by new arrival")
		query += " AND created_at >= NOW() - INTERVAL '30 days'"
	}

	// Add featured filter if requested
	if req.IsFeatured {
		utils.LogInfo("Filtering by featured")
		query += " AND is_featured = true"
	}

	// Get total count for pagination
	countQuery := `
		SELECT COUNT(*)
		FROM books 
		WHERE deleted_at IS NULL
	`

	// Check if admin is in context - if not, only count active books
	_, isAdmin = c.Get("admin")
	if !isAdmin {
		countQuery += " AND is_active = true"
	}

	// Add category filter if provided
	if req.CategoryID > 0 {
		countQuery += fmt.Sprintf(" AND category_id = %d", req.CategoryID)
	}

	// Add genre filter if provided
	if req.GenreID > 0 {
		countQuery += fmt.Sprintf(" AND genre_id = %d", req.GenreID)
	}

	// Add price range filters if provided
	if req.MinPrice > 0 {
		countQuery += fmt.Sprintf(" AND price >= %f", req.MinPrice)
	}
	if req.MaxPrice > 0 {
		countQuery += fmt.Sprintf(" AND price <= %f", req.MaxPrice)
	}

	// Add search filter if provided in count query
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		countQuery += fmt.Sprintf(" AND (books.name ILIKE '%s' OR books.author ILIKE '%s')",
			searchTerm, searchTerm)
	}

	// Add new arrival filter if requested
	if req.IsNewArrival {
		countQuery += " AND created_at >= NOW() - INTERVAL '30 days'"
	}

	// Add featured filter if requested
	if req.IsFeatured {
		countQuery += " AND is_featured = true"
	}

	var total int64
	if err := config.DB.Raw(countQuery).Scan(&total).Error; err != nil {
		utils.LogError("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to count books", err.Error())
		return
	}

	utils.LogInfo("Total books count: %d", total)

	// Add sorting
	switch req.SortBy {
	case "name":
		query += fmt.Sprintf(" ORDER BY books.name %s", req.Order)
	case "price":
		query += fmt.Sprintf(" ORDER BY books.price %s", req.Order)
	case "created_at":
		query += fmt.Sprintf(" ORDER BY books.created_at %s", req.Order)
	case "views":
		query += fmt.Sprintf(" ORDER BY books.views %s", req.Order)
	case "average_rating":
		query += fmt.Sprintf(" ORDER BY books.average_rating %s", req.Order)
	default:
		query += fmt.Sprintf(" ORDER BY books.created_at %s", req.Order)
	}

	utils.LogInfo("Sorting by: %s %s", req.SortBy, req.Order)

	// Add pagination
	offset := (req.Page - 1) * req.Limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", req.Limit, offset)

	utils.LogInfo("Pagination - Page: %d, Limit: %d, Offset: %d", req.Page, req.Limit, offset)

	// Execute the query
	var books []BookListItem
	if err := config.DB.Raw(query).Scan(&books).Error; err != nil {
		utils.LogError("Failed to fetch books: %v", err)
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	utils.LogInfo("Successfully fetched %d books", len(books))

	// Get categories for filtering with only essential fields
	type SimpleCategory struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var categories []SimpleCategory
	if err := config.DB.Raw("SELECT id, name, description FROM categories WHERE deleted_at IS NULL").Scan(&categories).Error; err != nil {
		utils.LogError("Failed to fetch categories: %v", err)
		// Continue anyway, as we have the books data
	}

	// Get genres for filtering with only essential fields
	type SimpleGenre struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var genres []SimpleGenre
	if err := config.DB.Raw("SELECT id, name, description FROM genres").Scan(&genres).Error; err != nil {
		utils.LogError("Failed to fetch genres: %v", err)
		// Continue anyway, as we have the books data
	}

	response := BookListResponse{
		Books: books,
		Pagination: gin.H{
			"total":       total,
			"page":        req.Page,
			"limit":       req.Limit,
			"total_pages": (total + int64(req.Limit) - 1) / int64(req.Limit),
		},
		Sort: gin.H{
			"by":    req.SortBy,
			"order": req.Order,
		},
		Filters: gin.H{
			"category_id": req.CategoryID,
			"genre_id":    req.GenreID,
		},
		AvailableFilters: gin.H{
			"categories": categories,
			"genres":     genres,
		},
	}

	utils.LogInfo("Successfully returning book list response with %d books", len(books))
	utils.Success(c, "Books retrieved successfully", response)
}
