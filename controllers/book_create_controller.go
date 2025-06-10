package controllers

import (
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

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

// CreateBook handles book creation
func CreateBook(c *gin.Context) {
	utils.LogInfo("CreateBook called")

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

	var req BookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}
	utils.LogDebug("Received book creation request - Name: %s, ISBN: %s", req.Name, req.ISBN)

	// Check if ISBN already exists
	var existingBook models.Book
	if err := config.DB.Where("isbn = ? AND deleted_at IS NULL", req.ISBN).First(&existingBook).Error; err == nil {
		utils.LogError("Book with ISBN %s already exists", req.ISBN)
		utils.Conflict(c, "A book with this ISBN already exists", gin.H{
			"isbn": req.ISBN,
		})
		return
	}
	utils.LogDebug("No existing book found with ISBN: %s", req.ISBN)

	// Validate category exists
	var category models.Category
	if err := config.DB.First(&category, req.CategoryID).Error; err != nil {
		utils.LogError("Category not found: %v", err)
		utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
		return
	}
	utils.LogDebug("Found category: %s", category.Name)

	// Validate genre exists
	var genre models.Genre
	if err := config.DB.First(&genre, req.GenreID).Error; err != nil {
		utils.LogError("Genre not found: %v", err)
		utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
		return
	}
	utils.LogDebug("Found genre: %s", genre.Name)

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
	utils.LogDebug("Created book model for: %s", book.Name)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to create book", tx.Error.Error())
		return
	}
	utils.LogDebug("Started database transaction")

	// Create the book within transaction
	if err := tx.Create(&book).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to create book: %v", err)

		if strings.Contains(err.Error(), "unique constraint") || strings.Contains(err.Error(), "duplicate key") {
			utils.Conflict(c, "A book with this ISBN already exists", gin.H{
				"isbn": req.ISBN,
			})
		} else {
			utils.InternalServerError(c, "Failed to create book", err.Error())
		}
		return
	}
	utils.LogDebug("Created book record with ID: %d", book.ID)

	// Insert BookImages if provided
	var bookImages []string
	if len(req.BookImages) > 0 {
		utils.LogDebug("Processing %d book images", len(req.BookImages))
		for _, url := range req.BookImages {
			if url != "" {
				if err := tx.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					tx.Rollback()
					utils.LogError("Failed to insert BookImage: %v", err)
					utils.InternalServerError(c, "Failed to create book images", err.Error())
					return
				}
				bookImages = append(bookImages, url)
			}
		}
		utils.LogDebug("Successfully added %d book images", len(bookImages))
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to create book", err.Error())
		return
	}
	utils.LogDebug("Successfully committed transaction")

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
	utils.LogDebug("Prepared response data for book: %s", book.Name)

	utils.LogInfo("Book created successfully: %s", book.Name)
	utils.Success(c, "Book created successfully", gin.H{
		"book": response,
	})
}
