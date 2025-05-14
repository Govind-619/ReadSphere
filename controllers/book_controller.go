package controllers

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
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
	BookImages         []string  `json:"images"`
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
			books.id, books.name, books.author, books.price, books.image_url, books.is_active, books.stock
		FROM books
		JOIN categories ON books.category_id = categories.id
		WHERE books.deleted_at IS NULL AND categories.deleted_at IS NULL
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
		query += fmt.Sprintf(" AND (books.name ILIKE '%s' OR books.author ILIKE '%s')",
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
		log.Printf("Failed to count books: %v", err)
		utils.InternalServerError(c, "Failed to count books", err.Error())
	}

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

	// Add pagination
	offset := (req.Page - 1) * req.Limit
	query += fmt.Sprintf(" LIMIT %d OFFSET %d", req.Limit, offset)

	// Execute the query
	var books []BookListItem
	if err := config.DB.Raw(query).Scan(&books).Error; err != nil {
		log.Printf("Failed to fetch books: %v", err)
		utils.InternalServerError(c, "Failed to fetch books", err.Error())
		return
	}

	// Get categories for filtering with only essential fields
	type SimpleCategory struct {
		ID          uint   `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	var categories []SimpleCategory
	if err := config.DB.Raw("SELECT id, name, description FROM categories WHERE deleted_at IS NULL").Scan(&categories).Error; err != nil {
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

	utils.Success(c, "Books retrieved successfully", response)
}

// CreateBook handles book creation
func CreateBook(c *gin.Context) {
	log.Printf("CreateBook called")

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

	var req BookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	// Check if ISBN already exists
	var existingBook models.Book
	if err := config.DB.Where("isbn = ? AND deleted_at IS NULL", req.ISBN).First(&existingBook).Error; err == nil {
		log.Printf("Book with ISBN %s already exists", req.ISBN)
		utils.Conflict(c, "A book with this ISBN already exists", gin.H{
			"isbn": req.ISBN,
		})
		return
	}

	// Validate category exists
	var category models.Category
	if err := config.DB.First(&category, req.CategoryID).Error; err != nil {
		log.Printf("Category not found: %v", err)
		utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
		return
	}

	// Validate genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, req.GenreID).Error; err != nil {
		log.Printf("Genre not found: %v", err)
		utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
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

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to create book", tx.Error.Error())
		return
	}

	// Create the book within transaction
	if err := tx.Create(&book).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to create book: %v", err)

		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			utils.Conflict(c, "A book with this ISBN already exists", gin.H{
				"isbn": req.ISBN,
			})
		} else {
			utils.InternalServerError(c, "Failed to create book", err.Error())
		}
		return
	}

	// Insert BookImages if provided
	var bookImages []string
	if len(req.BookImages) > 0 {
		for _, url := range req.BookImages {
			if url != "" {
				if err := tx.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					tx.Rollback()
					log.Printf("Failed to insert BookImage: %v", err)
					utils.InternalServerError(c, "Failed to create book images", err.Error())
					return
				}
				bookImages = append(bookImages, url)
			}
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to create book", err.Error())
		return
	}

	// Create a clean response without internal fields
	response := gin.H{
		"id":             book.ID,
		"name":           book.Name,
		"description":    book.Description,
		"price":          book.Price,
		"original_price": book.OriginalPrice,
		"stock":          book.Stock,
		"category": gin.H{
			"id":   category.ID,
			"name": category.Name,
		},
		"genre": gin.H{
			"id":   genre.ID,
			"name": genre.Name,
		},
		"image_url":        book.ImageURL,
		"images":           bookImages,
		"is_active":        book.IsActive,
		"is_featured":      book.IsFeatured,
		"author":           book.Author,
		"publisher":        book.Publisher,
		"isbn":             book.ISBN,
		"publication_year": book.PublicationYear,
		"pages":            book.Pages,
		"language":         book.Language,
		"format":           book.Format,
	}

	log.Printf("Book created successfully: %s", book.Name)
	utils.Success(c, "Book created successfully", gin.H{
		"book": response,
	})
}

// UpdateBook handles book updates
func UpdateBook(c *gin.Context) {
	log.Printf("UpdateBook called")

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

	// Get book ID from URL parameter
	bookID := c.Param("id")
	if bookID == "" {
		log.Printf("Book ID not provided in URL")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	log.Printf("Updating book with ID: %s", bookID)

	// Check if book exists
	var book models.Book
	if err := config.DB.First(&book, bookID).Error; err != nil {
		log.Printf("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	// Parse request body
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process update", nil)
		return
	}

	// Create a map to store fields to update
	updates := make(map[string]interface{})

	// Process each field in the update request
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
	if stock, ok := updateData["stock"].(float64); ok && stock >= 0 {
		updates["stock"] = int(stock)
	}
	if categoryID, ok := updateData["category_id"].(float64); ok && categoryID > 0 {
		// Verify category exists
		var category models.Category
		if err := tx.First(&category, uint(categoryID)).Error; err != nil {
			tx.Rollback()
			utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
			return
		}
		updates["category_id"] = uint(categoryID)
	}
	if genreID, ok := updateData["genre_id"].(float64); ok && genreID > 0 {
		// Verify genre exists
		var genre models.Genre
		if err := tx.First(&genre, uint(genreID)).Error; err != nil {
			tx.Rollback()
			utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
			return
		}
		updates["genre_id"] = uint(genreID)
	}
	if imageURL, ok := updateData["image_url"].(string); ok {
		updates["image_url"] = imageURL
	}
	if isActive, exists := updateData["is_active"].(bool); exists {
		updates["is_active"] = isActive
	}
	if isFeatured, exists := updateData["is_featured"].(bool); exists {
		updates["is_featured"] = isFeatured
	}
	if author, ok := updateData["author"].(string); ok && author != "" {
		updates["author"] = author
	}
	if publisher, ok := updateData["publisher"].(string); ok && publisher != "" {
		updates["publisher"] = publisher
	}
	if isbn, ok := updateData["isbn"].(string); ok && isbn != "" {
		// Check ISBN uniqueness
		var existingBook models.Book
		if err := tx.Where("isbn = ? AND id != ?", isbn, bookID).First(&existingBook).Error; err == nil {
			tx.Rollback()
			utils.Conflict(c, "A book with this ISBN already exists", nil)
			return
		}
		updates["isbn"] = isbn
	}
	if publicationYear, ok := updateData["publication_year"].(float64); ok && publicationYear > 0 {
		updates["publication_year"] = int(publicationYear)
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

	// Handle images array if provided
	if images, ok := updateData["images"].([]interface{}); ok {
		// Delete existing images
		if err := tx.Where("book_id = ?", bookID).Delete(&models.BookImage{}).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update images", nil)
			return
		}

		// Add new images
		for _, img := range images {
			if url, ok := img.(string); ok && url != "" {
				if err := tx.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					tx.Rollback()
					utils.InternalServerError(c, "Failed to add new images", nil)
					return
				}
			}
		}
	}

	// Update the book if there are changes
	if len(updates) > 0 {
		if err := tx.Model(&book).Updates(updates).Error; err != nil {
			tx.Rollback()
			log.Printf("Failed to update book: %v", err)
			utils.InternalServerError(c, "Failed to update book", nil)
			return
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to complete update", nil)
		return
	}

	// Fetch updated book details
	var updatedBook models.Book
	if err := config.DB.First(&updatedBook, bookID).Error; err != nil {
		utils.InternalServerError(c, "Book updated but failed to fetch details", nil)
		return
	}

	// Fetch category details
	var category models.Category
	if err := config.DB.First(&category, updatedBook.CategoryID).Error; err != nil {
		log.Printf("Failed to fetch category: %v", err)
	}

	// Fetch genre details
	var genre models.Genre
	if err := config.DB.First(&genre, updatedBook.GenreID).Error; err != nil {
		log.Printf("Failed to fetch genre: %v", err)
	}

	// Fetch book images
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&bookImages).Error; err != nil {
		log.Printf("Failed to fetch book images: %v", err)
	}

	// Create clean response
	response := gin.H{
		"id":               updatedBook.ID,
		"name":             updatedBook.Name,
		"description":      updatedBook.Description,
		"price":            updatedBook.Price,
		"original_price":   updatedBook.OriginalPrice,
		"stock":            updatedBook.Stock,
		"image_url":        updatedBook.ImageURL,
		"is_active":        updatedBook.IsActive,
		"is_featured":      updatedBook.IsFeatured,
		"author":           updatedBook.Author,
		"publisher":        updatedBook.Publisher,
		"isbn":             updatedBook.ISBN,
		"publication_year": updatedBook.PublicationYear,
		"pages":            updatedBook.Pages,
		"language":         updatedBook.Language,
		"format":           updatedBook.Format,
		"category": gin.H{
			"id":   category.ID,
			"name": category.Name,
		},
		"genre": gin.H{
			"id":   genre.ID,
			"name": genre.Name,
		},
	}

	// Add images to response
	imageUrls := make([]string, 0)
	for _, img := range bookImages {
		imageUrls = append(imageUrls, img.URL)
	}
	response["images"] = imageUrls

	log.Printf("Book updated successfully: %s", updatedBook.Name)
	utils.Success(c, "Book updated successfully", gin.H{
		"book": response,
	})
}

// DeleteBook handles book deletion
func DeleteBook(c *gin.Context) {
	log.Printf("DeleteBook called")

	// Get the ID from the URL parameter
	id := c.Param("id")
	if id == "" {
		log.Printf("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	log.Printf("Attempting to delete book with ID: %s", id)

	// Check if the book exists
	var book models.Book
	if err := config.DB.First(&book, id).Error; err != nil {
		log.Printf("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		log.Printf("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process deletion", nil)
		return
	}

	// Delete associated book images first
	if err := tx.Where("book_id = ?", id).Delete(&models.BookImage{}).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to delete book images: %v", err)
		utils.InternalServerError(c, "Failed to delete book images", nil)
		return
	}

	// Delete the book
	if err := tx.Delete(&book).Error; err != nil {
		tx.Rollback()
		log.Printf("Failed to delete book: %v", err)
		utils.InternalServerError(c, "Failed to delete book", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to complete deletion", nil)
		return
	}

	log.Printf("Book deleted successfully: %s (ID: %s)", book.Name, id)
	utils.Success(c, "Book deleted successfully", nil)
}

// GetBookReviews handles fetching reviews for a book
func GetBookReviews(c *gin.Context) {
	bookID := c.Param("id")
	var reviews []models.Review

	if err := config.DB.Preload("User").Where("book_id = ?", bookID).Find(&reviews).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch reviews", err.Error())
	}

	utils.Success(c, "Reviews retrieved successfully", reviews)
}

// ApproveReview handles approving a book review
func ApproveReview(c *gin.Context) {
	reviewID := c.Param("reviewId")
	var review models.Review

	if err := config.DB.First(&review, reviewID).Error; err != nil {
		utils.NotFound(c, "Review not found")
	}

	review.IsApproved = true
	if err := config.DB.Save(&review).Error; err != nil {
		utils.InternalServerError(c, "Failed to approve review", err.Error())
	}

	utils.Success(c, "Review approved successfully", review)
}

// DeleteReview handles deleting a book review
func DeleteReview(c *gin.Context) {
	reviewID := c.Param("reviewId")
	var review models.Review

	if err := config.DB.First(&review, reviewID).Error; err != nil {
		utils.NotFound(c, "Review not found")
	}

	if err := config.DB.Delete(&review).Error; err != nil {
		utils.InternalServerError(c, "Failed to delete review", err.Error())
	}

	utils.Success(c, "Review deleted successfully", gin.H{"message": "Review deleted successfully"})
}

// GetBookDetails retrieves details of a specific book
func GetBookDetails(c *gin.Context) {
	log.Printf("GetBookDetails called")

	bookID := c.Param("id")
	log.Printf("Book ID: %s", bookID)

	if bookID == "" {
		log.Printf("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
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
		utils.NotFound(c, "Book not found")
		return
	}

	// Now fetch the images from book_images table
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&bookImages).Error; err != nil {
		log.Printf("Failed to fetch images for book %s: %v", bookID, err)
	} else {
		book.BookImages = bookImages
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

	// Check if user is admin
	_, isAdmin := c.Get("admin")

	// Create response based on user role
	response := gin.H{
		"book": gin.H{
			"id":               book.ID,
			"name":             book.Name,
			"description":      book.Description,
			"price":            book.Price,
			"original_price":   book.OriginalPrice,
			"stock":            book.Stock,
			"image_url":        book.ImageURL,
			"images":           bookImages,
			"author":           book.Author,
			"publisher":        book.Publisher,
			"isbn":             book.ISBN,
			"publication_year": book.PublicationYear,
			"pages":            book.Pages,
			"language":         book.Language,
			"format":           book.Format,
			"created_at":       book.CreatedAt,
			"updated_at":       book.UpdatedAt,
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
		},
	}

	if isAdmin {
		// Add admin-specific fields
		bookData := response["book"].(gin.H)
		bookData["is_active"] = book.IsActive
		bookData["is_featured"] = book.IsFeatured
		bookData["views"] = book.Views
		bookData["average_rating"] = book.AverageRating
		bookData["total_reviews"] = book.TotalReviews
		bookData["blocked"] = book.Blocked
	}

	log.Printf("Book found: %s (ID: %s)", book.Name, bookID)
	utils.Success(c, "Book retrieved successfully", response)
}

// CheckBookExists checks if a book exists by ID
func CheckBookExists(c *gin.Context) {
	id := c.Param("id")
	log.Printf("CheckBookExists called with ID: %s", id)

	if id == "" {
		log.Printf("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
	}

	// First, fetch the book without the images field
	var book models.Book
	if err := config.DB.First(&book, id).Error; err != nil {
		log.Printf("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
	}

	// Fetch BookImages from the BookImage table
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", book.ID).Find(&bookImages).Error; err != nil {
		log.Printf("Failed to fetch BookImages for book %s: %v", id, err)
	}
	// Build images array
	images := []string{}
	for _, img := range bookImages {
		images = append(images, img.URL)
	}

	log.Printf("Book found: %s (ID: %s)", book.Name, id)
	utils.Success(c, "Book found", gin.H{
		"message": "Book found",
		"book":    book,
		"images":  images,
	})
}
