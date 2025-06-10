package controllers

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

func GetBookDetails(c *gin.Context) {
	utils.LogInfo("GetBookDetails called")

	bookID := c.Param("id")
	utils.LogInfo("Book ID: %s", bookID)

	if bookID == "" {
		utils.LogError("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	// First, check if the book is blocked for non-admin users
	_, isAdmin := c.Get("admin")
	utils.LogInfo("User is admin: %v", isAdmin)

	if !isAdmin {
		// Debug query to check book status
		var bookStatus struct {
			Blocked   bool       `gorm:"column:blocked"`
			IsActive  bool       `gorm:"column:is_active"`
			DeletedAt *time.Time `gorm:"column:deleted_at"`
		}

		debugQuery := "SELECT blocked, is_active, deleted_at FROM books WHERE id = ?"
		if err := config.DB.Raw(debugQuery, bookID).Scan(&bookStatus).Error; err != nil {
			utils.LogError("Failed to check book status: %v", err)
			utils.InternalServerError(c, "Failed to verify book status", err.Error())
			return
		}

		utils.LogInfo("Book %s status - Blocked: %v, Active: %v, Deleted: %v",
			bookID, bookStatus.Blocked, bookStatus.IsActive, bookStatus.DeletedAt != nil)

		if bookStatus.Blocked {
			utils.LogError("Access denied: Book %s is blocked", bookID)
			utils.Forbidden(c, "This book is not available")
			return
		}

		if !bookStatus.IsActive {
			utils.LogError("Access denied: Book %s is not active", bookID)
			utils.Forbidden(c, "This book is not available")
			return
		}

		if bookStatus.DeletedAt != nil {
			utils.LogError("Access denied: Book %s is deleted", bookID)
			utils.NotFound(c, "Book not found")
			return
		}
	}

	// Now fetch the book details
	var book models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, original_price, discount_percentage, discount_end_date, stock, category_id, 
			genre_id, image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages, language, format,
			blocked
		FROM books 
		WHERE id = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, bookID).Scan(&book).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	utils.LogInfo("Book found in database: %s (ID: %s, Blocked: %v, Active: %v)",
		book.Name, bookID, book.Blocked, book.IsActive)

	// Now fetch the images from book_images table
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&bookImages).Error; err != nil {
		utils.LogError("Failed to fetch images for book %s: %v", bookID, err)
	} else {
		book.BookImages = bookImages
		utils.LogInfo("Successfully fetched %d images for book %s", len(bookImages), bookID)
	}

	// Load category using raw SQL
	var category models.Category
	if err := config.DB.Raw("SELECT * FROM categories WHERE id = ?", book.CategoryID).Scan(&category).Error; err != nil {
		utils.LogError("Failed to load category: %v", err)
	} else {
		book.Category = category
		utils.LogInfo("Successfully loaded category: %s for book %s", category.Name, bookID)
	}

	// Load genre using raw SQL
	var genre models.Genre
	if err := config.DB.Raw("SELECT * FROM genres WHERE id = ?", book.GenreID).Scan(&genre).Error; err != nil {
		utils.LogError("Failed to load genre: %v", err)
	} else {
		book.Genre = genre
		utils.LogInfo("Successfully loaded genre: %s for book %s", genre.Name, bookID)
	}

	if isAdmin {
		utils.LogInfo("Admin access detected for book %s", bookID)
	}

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
		utils.LogInfo("Added admin-specific fields for book %s", bookID)
	}

	utils.LogInfo("Successfully prepared book details response for book %s", bookID)
	utils.Success(c, "Book retrieved successfully", response)
}

// CheckBookExists checks if a book exists by ID
func CheckBookExists(c *gin.Context) {
	id := c.Param("id")
	utils.LogInfo("CheckBookExists called with ID: %s", id)

	if id == "" {
		utils.LogError("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	// First, fetch the book without the images field
	var book models.Book
	if err := config.DB.First(&book, id).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	utils.LogInfo("Book found in database: %s (ID: %s)", book.Name, id)

	// Fetch BookImages from the BookImage table
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", book.ID).Find(&bookImages).Error; err != nil {
		utils.LogError("Failed to fetch BookImages for book %s: %v", id, err)
	} else {
		utils.LogInfo("Successfully fetched %d images for book %s", len(bookImages), id)
	}

	// Build images array
	images := []string{}
	for _, img := range bookImages {
		images = append(images, img.URL)
	}

	utils.LogInfo("Successfully prepared book existence check response for book %s", id)
	utils.Success(c, "Book found", gin.H{
		"message": "Book found",
		"book":    book,
		"images":  images,
	})
}
