package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// BookRequest represents the book creation/update request
type BookRequest struct {
	Name               string    `json:"name" binding:"required"`
	Description        string    `json:"description" binding:"required"`
	Price              float64   `json:"price" binding:"required,min=0"` // Price in local currency
	OriginalPrice      float64   `json:"original_price"`
	DiscountPercentage int       `json:"discount_percentage"`
	DiscountEndDate    time.Time `json:"discount_end_date"`
	Stock              int       `json:"stock" binding:"required,min=0"`
	CategoryID         uint      `json:"category_id" binding:"required"`
	GenreID            uint      `json:"genre_id" binding:"required"`
	ImageURL           string    `json:"image_url"`
	Images             []string  `json:"images"`
	IsActive           bool      `json:"is_active"`
	IsFeatured         bool      `json:"is_featured"`
	Author             string    `json:"author" binding:"required"`
	Publisher          string    `json:"publisher" binding:"required"`
	ISBN               string    `json:"isbn" binding:"required"`
	PublicationYear    int       `json:"publication_year" binding:"required"`
	Pages              int       `json:"pages" binding:"required,min=1"`
	Language           string    `json:"language"`
	Format             string    `json:"format"`
}

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

// GetBooks handles listing books with search, pagination, and sorting
func GetBooks(c *gin.Context) {
	log.Printf("GetBooks called")

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
			id, name, author, price, image_url, is_active, stock
		FROM books 
		WHERE deleted_at IS NULL
	`

	// Check if admin is in context - if not, only show active books
	_, isAdmin := c.Get("admin")
	if !isAdmin {
		query += " AND is_active = true"
	}

	// Add category filter if provided
	if req.CategoryID > 0 {
		log.Printf("Filtering by category_id: %d", req.CategoryID)
		query += fmt.Sprintf(" AND category_id = %d", req.CategoryID)
	}

	// Add genre filter if provided
	if req.GenreID > 0 {
		log.Printf("Filtering by genre_id: %d", req.GenreID)
		query += fmt.Sprintf(" AND genre_id = %d", req.GenreID)
	}

	// Add price range filters if provided
	if req.MinPrice > 0 {
		log.Printf("Filtering by min_price: %f", req.MinPrice)
		query += fmt.Sprintf(" AND price >= %f", req.MinPrice)
	}
	if req.MaxPrice > 0 {
		log.Printf("Filtering by max_price: %f", req.MaxPrice)
		query += fmt.Sprintf(" AND price <= %f", req.MaxPrice)
	}

	// Add search filter if provided
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		log.Printf("Filtering by search term: %s", req.Search)
		query += fmt.Sprintf(" AND (name ILIKE '%s' OR author ILIKE '%s')",
			searchTerm, searchTerm)
	}

	// Add new arrival filter if requested
	if req.IsNewArrival {
		log.Printf("Filtering by new arrival")
		query += " AND created_at >= NOW() - INTERVAL '30 days'"
	}

	// Add featured filter if requested
	if req.IsFeatured {
		log.Printf("Filtering by featured")
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

	// Add search filter if provided
	if req.Search != "" {
		searchTerm := "%" + req.Search + "%"
		countQuery += fmt.Sprintf(" AND (name ILIKE '%s' OR author ILIKE '%s')",
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
		log.Printf("Failed to count books: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count books"})
		return
	}

	// Add sorting
	switch req.SortBy {
	case "name":
		query += fmt.Sprintf(" ORDER BY name %s", req.Order)
	case "price":
		query += fmt.Sprintf(" ORDER BY price %s", req.Order)
	case "created_at":
		query += fmt.Sprintf(" ORDER BY created_at %s", req.Order)
	case "views":
		query += fmt.Sprintf(" ORDER BY views %s", req.Order)
	case "average_rating":
		query += fmt.Sprintf(" ORDER BY average_rating %s", req.Order)
	default:
		query += fmt.Sprintf(" ORDER BY created_at %s", req.Order)
	}

	// Add pagination
	offset := (req.Page - 1) * req.Limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", req.Limit, offset)

	// Execute the query
	var books []BookListItem
	if err := config.DB.Raw(query).Scan(&books).Error; err != nil {
		log.Printf("Failed to fetch books: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch books"})
		return
	}

	// Get categories for filtering with only essential fields
	type SimpleCategory struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var categories []SimpleCategory
	if err := config.DB.Raw("SELECT id, name, description FROM categories").Scan(&categories).Error; err != nil {
		log.Printf("Failed to fetch categories: %v", err)
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
		log.Printf("Failed to fetch genres: %v", err)
		// Continue anyway, as we have the books data
	}

	c.JSON(http.StatusOK, gin.H{
		"books": books,
		"pagination": gin.H{
			"total":       total,
			"page":        req.Page,
			"limit":       req.Limit,
			"total_pages": (total + int64(req.Limit) - 1) / int64(req.Limit),
		},
		"filters": gin.H{
			"category_id": req.CategoryID,
			"genre_id":    req.GenreID,
		},
		"sort": gin.H{
			"by":    req.SortBy,
			"order": req.Order,
		},
		"available_filters": gin.H{
			"categories": categories,
			"genres":     genres,
		},
	})
}

// validateImageURLs validates a list of image URLs
func validateImageURLs(urls []string) error {
	if len(urls) == 0 {
		return nil // Empty array is valid
	}

	for _, url := range urls {
		if url == "" {
			continue // Skip empty URLs
		}

		// Check if the URL starts with http:// or https://
		if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
			return fmt.Errorf("invalid image URL: %s (must start with http:// or https://)", url)
		}

		// Check if the URL ends with a common image extension
		validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
		hasValidExtension := false
		for _, ext := range validExtensions {
			if strings.HasSuffix(strings.ToLower(url), ext) {
				hasValidExtension = true
				break
			}
		}

		if !hasValidExtension {
			return fmt.Errorf("invalid image URL: %s (must end with a valid image extension)", url)
		}
	}

	return nil
}

// CreateBook handles book creation
func CreateBook(c *gin.Context) {
	log.Printf("CreateBook called")

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

	var req BookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if category exists
	if err := EnsureCategoryExists(req.CategoryID); err != nil {
		log.Printf("Category validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if genre exists
	if err := EnsureGenreExists(req.GenreID); err != nil {
		log.Printf("Genre validation failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create the book
	book := models.Book{
		Name:               req.Name,
		Description:        req.Description,
		Price:              req.Price,
		OriginalPrice:      req.OriginalPrice,
		DiscountPercentage: req.DiscountPercentage,
		DiscountEndDate:    req.DiscountEndDate,
		Stock:              req.Stock,
		CategoryID:         req.CategoryID,
		GenreID:            req.GenreID,
		ImageURL:           req.ImageURL,
		Images:             strings.Join(req.Images, ","),
		IsActive:           req.IsActive,
		IsFeatured:         req.IsFeatured,
		Author:             req.Author,
		Publisher:          req.Publisher,
		ISBN:               req.ISBN,
		PublicationYear:    req.PublicationYear,
		Pages:              req.Pages,
		Language:           req.Language,
		Format:             req.Format,
	}

	if err := config.DB.Create(&book).Error; err != nil {
		log.Printf("Failed to create book: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create book"})
		return
	}

	log.Printf("Book created successfully: %s", book.Name)
	c.JSON(http.StatusCreated, gin.H{
		"message": "Book created successfully",
		"book":    book,
	})
}

// UpdateBook handles book updates
func UpdateBook(c *gin.Context) {
	log.Printf("UpdateBook called")

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

	// Get book ID from URL parameter
	bookID := c.Param("id")
	if bookID == "" {
		log.Printf("Book ID not provided in URL")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book ID is required"})
		return
	}

	log.Printf("Updating book with ID: %s", bookID)

	// Check if book exists - use a raw SQL query to avoid the scanning error with the images column
	var book models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, original_price, discount_percentage, discount_end_date, stock, category_id, 
			genre_id, image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages, language, format
		FROM books 
		WHERE id = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, bookID).Scan(&book).Error; err != nil {
		log.Printf("Book not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	log.Printf("Found book to update: %s (ID: %s)", book.Name, bookID)

	// Parse request body into a map
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		log.Printf("Invalid input: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Update data: %+v", updateData)

	// Create a map to store fields to update
	updates := make(map[string]interface{})

	// Check each field in the request and update only those that are provided
	if name, ok := updateData["name"].(string); ok && name != "" {
		updates["name"] = name
	}

	if description, ok := updateData["description"].(string); ok && description != "" {
		updates["description"] = description
	}

	if price, ok := updateData["price"].(float64); ok && price > 0 {
		updates["price"] = price
	}

	if originalPrice, ok := updateData["original_price"].(float64); ok && originalPrice > 0 {
		updates["original_price"] = originalPrice
	}

	if discountPercentage, ok := updateData["discount_percentage"].(float64); ok && discountPercentage >= 0 {
		updates["discount_percentage"] = int(discountPercentage)
	}

	if discountEndDate, ok := updateData["discount_end_date"].(string); ok && discountEndDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, discountEndDate)
		if err != nil {
			log.Printf("Invalid discount end date format: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid discount end date format"})
			return
		}
		updates["discount_end_date"] = parsedDate
	}

	if stock, ok := updateData["stock"].(float64); ok && stock >= 0 {
		updates["stock"] = int(stock)
	}

	if categoryID, ok := updateData["category_id"].(float64); ok && categoryID > 0 {
		// Ensure category exists
		if err := EnsureCategoryExists(uint(categoryID)); err != nil {
			log.Printf("Failed to ensure category exists: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure category exists"})
			return
		}
		updates["category_id"] = uint(categoryID)
	}

	if imageURL, ok := updateData["image_url"].(string); ok && imageURL != "" {
		updates["image_url"] = imageURL
	}

	// Handle images array separately
	if images, ok := updateData["images"].([]interface{}); ok && len(images) > 0 {
		// Convert []interface{} to []string
		imageURLs := make([]string, len(images))
		for i, img := range images {
			if url, ok := img.(string); ok {
				imageURLs[i] = url
			}
		}

		// Validate image URLs
		if err := validateImageURLs(imageURLs); err != nil {
			log.Printf("Invalid image URLs: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Format the array for PostgreSQL
		imagesArray := "ARRAY["
		for i, img := range imageURLs {
			if i > 0 {
				imagesArray += ", "
			}
			imagesArray += fmt.Sprintf("'%s'", strings.ReplaceAll(img, "'", "''"))
		}
		imagesArray += "]::text[]"

		// Update the images field using a raw SQL query
		if err := config.DB.Exec("UPDATE books SET images = "+imagesArray+" WHERE id = ?", bookID).Error; err != nil {
			log.Printf("Failed to update images: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update images: " + err.Error()})
			return
		}
	}

	if isActive, ok := updateData["is_active"].(bool); ok {
		updates["is_active"] = isActive
	}

	if isFeatured, ok := updateData["is_featured"].(bool); ok {
		updates["is_featured"] = isFeatured
	}

	if author, ok := updateData["author"].(string); ok && author != "" {
		updates["author"] = author
	}

	if publisher, ok := updateData["publisher"].(string); ok && publisher != "" {
		updates["publisher"] = publisher
	}

	if isbn, ok := updateData["isbn"].(string); ok && isbn != "" {
		// Check if ISBN already exists in an active book (excluding the current book)
		var existingBook models.Book
		if err := config.DB.Where("isbn = ? AND id != ? AND deleted_at IS NULL", isbn, bookID).First(&existingBook).Error; err == nil {
			log.Printf("ISBN already exists in an active book: %s", isbn)
			c.JSON(http.StatusBadRequest, gin.H{"error": "A book with this ISBN already exists"})
			return
		}
		updates["isbn"] = isbn
	}

	if publicationYear, ok := updateData["publication_year"].(float64); ok && publicationYear > 0 {
		updates["publication_year"] = int(publicationYear)
	}

	if genreID, ok := updateData["genre_id"].(float64); ok && genreID > 0 {
		// Ensure genre exists
		if err := EnsureGenreExists(uint(genreID)); err != nil {
			log.Printf("Failed to ensure genre exists: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to ensure genre exists"})
			return
		}
		updates["genre_id"] = uint(genreID)
	}

	if pages, ok := updateData["pages"].(float64); ok && pages > 0 {
		updates["pages"] = int(pages)
	}

	if language, ok := updateData["language"].(string); ok && language != "" {
		updates["language"] = language
	}

	if format, ok := updateData["format"].(string); ok && format != "" {
		updates["format"] = format
	}

	// Update the book with only the provided fields (excluding images which we already updated)
	if len(updates) > 0 {
		log.Printf("Updating book with fields: %+v", updates)
		if err := config.DB.Model(&book).Updates(updates).Error; err != nil {
			log.Printf("Failed to update book: %v", err)

			// Check for unique constraint violation (ISBN)
			if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
				log.Printf("ISBN already exists: %s", updates["isbn"])
				c.JSON(http.StatusBadRequest, gin.H{"error": "A book with this ISBN already exists"})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book: " + err.Error()})
			return
		}
	}

	// Fetch the updated book with explicit error handling
	var updatedBook models.Book
	query = `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, original_price, discount_percentage, discount_end_date, stock, category_id, 
			genre_id, image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages, language, format
		FROM books 
		WHERE id = ?
	`

	if err := config.DB.Raw(query, bookID).Scan(&updatedBook).Error; err != nil {
		log.Printf("Failed to fetch updated book: %v", err)
		// Return the book we updated, even if we couldn't fetch the updated version
		c.JSON(http.StatusOK, gin.H{
			"message": "Book updated successfully, but failed to fetch updated version",
			"book":    book,
		})
		return
	}

	// Now fetch the images separately
	var images []string
	if err := config.DB.Raw("SELECT images FROM books WHERE id = ?", bookID).Scan(&images).Error; err != nil {
		log.Printf("Failed to fetch images for book %s: %v", bookID, err)
		// Continue anyway, as we have the book data
	} else {
		updatedBook.Images = strings.Join(images, ",")
	}

	log.Printf("Book updated successfully: %s", updatedBook.Name)
	c.JSON(http.StatusOK, gin.H{
		"message": "Book updated successfully",
		"book":    updatedBook,
	})
}

// DeleteBook handles book deletion
func DeleteBook(c *gin.Context) {
	log.Printf("DeleteBook called")

	// First try to get the ID from the URL parameter
	id := c.Param("id")
	log.Printf("Book ID from URL parameter: %s", id)

	// If the ID is not in the URL, try to get it from the request body
	if id == "" {
		var req struct {
			ID string `json:"ID"`
		}
		if err := c.ShouldBindJSON(&req); err == nil && req.ID != "" {
			id = req.ID
			log.Printf("Book ID from request body: %s", id)
		}
	}

	if id == "" {
		log.Printf("Book ID is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book ID is required"})
		return
	}

	// Check if the book exists before trying to delete it
	// Use a raw SQL query to avoid the scanning error with the images column
	var book models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, stock, category_id, 
			image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages
		FROM books 
		WHERE id = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, id).Scan(&book).Error; err != nil {
		log.Printf("Book not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	log.Printf("Found book to delete: %s (ID: %s)", book.Name, id)

	// Use Unscoped().Delete to ensure the book is completely removed from the database
	// This will allow the ISBN to be reused
	if err := config.DB.Unscoped().Delete(&book).Error; err != nil {
		log.Printf("Failed to delete book: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book: " + err.Error()})
		return
	}

	log.Printf("Book deleted successfully: %s (ID: %s)", book.Name, id)
	c.JSON(http.StatusOK, gin.H{"message": "Book deleted successfully"})
}

// GetBookReviews handles fetching reviews for a book
func GetBookReviews(c *gin.Context) {
	bookID := c.Param("id")
	var reviews []models.Review

	if err := config.DB.Preload("User").Where("book_id = ?", bookID).Find(&reviews).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch reviews"})
		return
	}

	c.JSON(http.StatusOK, reviews)
}

// ApproveReview handles approving a book review
func ApproveReview(c *gin.Context) {
	reviewID := c.Param("reviewId")
	var review models.Review

	if err := config.DB.First(&review, reviewID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
		return
	}

	review.IsApproved = true
	if err := config.DB.Save(&review).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to approve review"})
		return
	}

	c.JSON(http.StatusOK, review)
}

// DeleteReview handles deleting a book review
func DeleteReview(c *gin.Context) {
	reviewID := c.Param("reviewId")
	var review models.Review

	if err := config.DB.First(&review, reviewID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Review not found"})
		return
	}

	if err := config.DB.Delete(&review).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete review"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Review deleted successfully"})
}

// GetBookDetails retrieves details of a specific book
func GetBookDetails(c *gin.Context) {
	log.Printf("GetBookDetails called")

	bookID := c.Param("id")
	log.Printf("Book ID: %s", bookID)

	if bookID == "" {
		log.Printf("Book ID is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book ID is required"})
		return
	}

	// First, fetch the book without the images field
	var book models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, original_price, discount_percentage, discount_end_date, stock, category_id, 
			genre_id, image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages, language, format
		FROM books 
		WHERE id = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, bookID).Scan(&book).Error; err != nil {
		log.Printf("Book not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found: " + err.Error()})
		return
	}

	// Check if the book is inactive
	if !book.IsActive {
		log.Printf("Book is inactive: %s (ID: %s)", book.Name, bookID)
		c.JSON(http.StatusFound, gin.H{
			"error":        "Book is inactive. Redirected to products page.",
			"redirect_url": "/v1/books",
		})
		return
	}

	// Now fetch the images separately using array_to_json
	var imagesJSON string
	if err := config.DB.Raw("SELECT COALESCE(array_to_json(images), '[]'::json) FROM books WHERE id = ?", bookID).Scan(&imagesJSON).Error; err != nil {
		log.Printf("Failed to fetch images for book %s: %v", bookID, err)
	} else {
		if err := json.Unmarshal([]byte(imagesJSON), &book.Images); err != nil {
			log.Printf("Failed to parse images JSON for book %s: %v", bookID, err)
		}
	}

	// Load category using raw SQL
	var category models.Category
	if err := config.DB.Raw("SELECT * FROM categories WHERE id = ?", book.CategoryID).Scan(&category).Error; err != nil {
		log.Printf("Failed to load category: %v", err)
	} else {
		book.Category = category
	}

	// Load genre using raw SQL
	var genre models.Genre
	if err := config.DB.Raw("SELECT * FROM genres WHERE id = ?", book.GenreID).Scan(&genre).Error; err != nil {
		log.Printf("Failed to load genre: %v", err)
	} else {
		book.Genre = genre
	}

	// Create breadcrumbs
	breadcrumbs := []gin.H{
		{"name": "Home", "url": "/"},
		{"name": "Books", "url": "/books"},
		{"name": book.Category.Name, "url": fmt.Sprintf("/books?category_id=%d", book.CategoryID)},
		{"name": book.Name, "url": ""},
	}

	// Fetch rating distribution
	type RatingDistribution struct {
		FiveStars  int `json:"five_stars"`
		FourStars  int `json:"four_stars"`
		ThreeStars int `json:"three_stars"`
		TwoStars   int `json:"two_stars"`
		OneStar    int `json:"one_star"`
	}

	var ratingDist RatingDistribution
	if err := config.DB.Raw(`
		SELECT 
			COUNT(CASE WHEN rating = 5 THEN 1 END) as five_stars,
			COUNT(CASE WHEN rating = 4 THEN 1 END) as four_stars,
			COUNT(CASE WHEN rating = 3 THEN 1 END) as three_stars,
			COUNT(CASE WHEN rating = 2 THEN 1 END) as two_stars,
			COUNT(CASE WHEN rating = 1 THEN 1 END) as one_star
		FROM reviews 
		WHERE book_id = ? AND is_approved = true
	`, bookID).Scan(&ratingDist).Error; err != nil {
		log.Printf("Failed to fetch rating distribution: %v", err)
	}

	// Calculate discounted price
	hasDiscount := book.DiscountPercentage > 0 && time.Now().Before(book.DiscountEndDate)
	if hasDiscount {
		book.Price = book.OriginalPrice * (1 - float64(book.DiscountPercentage)/100)
	}

	// Fetch recent reviews
	var reviews []models.Review
	if err := config.DB.Preload("User").Where("book_id = ? AND is_approved = true", bookID).Order("created_at desc").Limit(5).Find(&reviews).Error; err != nil {
		log.Printf("Failed to fetch reviews: %v", err)
	}

	// Determine stock status and message
	var stockStatus, stockMessage string
	if book.Stock <= 0 {
		stockStatus = "Out of Stock"
		stockMessage = "This book is currently out of stock. Please check back later."
	} else if book.Stock < 5 {
		stockStatus = "Low Stock"
		stockMessage = fmt.Sprintf("Only %d copies left in stock!", book.Stock)
	} else {
		stockStatus = "In Stock"
		stockMessage = fmt.Sprintf("%d copies available", book.Stock)
	}

	// Create product specifications
	specs := []gin.H{
		{"key": "Author", "value": book.Author},
		{"key": "Publisher", "value": book.Publisher},
		{"key": "ISBN", "value": book.ISBN},
		{"key": "Publication Year", "value": strconv.Itoa(book.PublicationYear)},
		{"key": "Pages", "value": strconv.Itoa(book.Pages)},
		{"key": "Language", "value": book.Language},
		{"key": "Format", "value": book.Format},
	}

	// Fetch related books
	var relatedBooks []models.Book
	if err := config.DB.Where("category_id = ? AND id != ? AND is_active = true AND deleted_at IS NULL",
		book.CategoryID, bookID).Limit(4).Find(&relatedBooks).Error; err != nil {
		log.Printf("Failed to fetch related books: %v", err)
	}

	// Increment view count
	config.DB.Model(&book).UpdateColumn("views", book.Views+1)

	// Check if user is admin
	_, isAdmin := c.Get("admin")

	// Create response based on user role
	response := gin.H{
		"book":                book,
		"breadcrumbs":         breadcrumbs,
		"rating_distribution": ratingDist,
		"has_discount":        hasDiscount,
		"discount_end_date":   book.DiscountEndDate,
		"reviews":             reviews,
		"total_reviews":       book.TotalReviews,
		"stock_status":        stockStatus,
		"stock_message":       stockMessage,
		"specifications":      specs,
		"related_books":       relatedBooks,
	}

	// If not admin, remove sensitive fields
	if !isAdmin {
		// Create a simplified book response without sensitive fields
		simplifiedBook := gin.H{
			"id":               book.ID,
			"name":             book.Name,
			"description":      book.Description,
			"price":            book.Price,
			"image_url":        book.ImageURL,
			"images":           book.Images,
			"author":           book.Author,
			"publisher":        book.Publisher,
			"isbn":             book.ISBN,
			"publication_year": book.PublicationYear,
			"pages":            book.Pages,
			"language":         book.Language,
			"format":           book.Format,
			"stock":            book.Stock,
			"category": gin.H{
				"id":          book.Category.ID,
				"name":        book.Category.Name,
				"description": book.Category.Description,
			},
			"genre": gin.H{
				"id":          book.Genre.ID,
				"name":        book.Genre.Name,
				"description": book.Genre.Description,
			},
		}
		response["book"] = simplifiedBook

		// Simplify related books
		var simplifiedRelatedBooks []gin.H
		for _, rb := range relatedBooks {
			simplifiedRelatedBooks = append(simplifiedRelatedBooks, gin.H{
				"id":          rb.ID,
				"name":        rb.Name,
				"description": rb.Description,
				"price":       rb.Price,
				"image_url":   rb.ImageURL,
				"author":      rb.Author,
			})
		}
		response["related_books"] = simplifiedRelatedBooks
	}

	log.Printf("Book found: %s (ID: %s)", book.Name, bookID)
	c.JSON(http.StatusOK, response)
}

// CheckBookExists checks if a book exists by ID
func CheckBookExists(c *gin.Context) {
	id := c.Param("id")
	log.Printf("CheckBookExists called with ID: %s", id)

	if id == "" {
		log.Printf("Book ID is empty")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book ID is required"})
		return
	}

	// First, fetch the book without the images field
	var book models.Book
	if err := config.DB.First(&book, id).Error; err != nil {
		log.Printf("Book not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	// Now fetch the images separately
	var imagesJSON string
	if err := config.DB.Raw("SELECT COALESCE(array_to_json(images), '[]'::json) FROM books WHERE id = ?", id).Scan(&imagesJSON).Error; err != nil {
		log.Printf("Failed to fetch images for book %s: %v", id, err)
		// Continue anyway, as we have the book data
	} else {
		// Parse the JSON string into []string
		if err := json.Unmarshal([]byte(imagesJSON), &book.Images); err != nil {
			log.Printf("Failed to parse images JSON for book %s: %v", id, err)
		}
	}

	log.Printf("Book found: %s (ID: %s)", book.Name, id)
	c.JSON(http.StatusOK, gin.H{
		"message": "Book found",
		"book":    book,
	})
}
