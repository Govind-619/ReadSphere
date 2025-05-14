package controllers

import (
	"fmt"
	"log"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UpdateBookByField handles book updates by any unique field
func UpdateBookByField(c *gin.Context) {
	log.Printf("UpdateBookByField called")

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

	// Get field name and value from URL parameters
	fieldName := c.Param("field")
	fieldValue := c.Param("value")

	if fieldName == "" || fieldValue == "" {
		log.Printf("Field name or value not provided in URL")
		utils.BadRequest(c, "Field name and value are required", nil)
		return
	}

	log.Printf("Updating book with %s: %s", fieldName, fieldValue)

	// Check if book exists
	var book models.Book
	if err := config.DB.Where(fmt.Sprintf("%s = ?", fieldName), fieldValue).First(&book).Error; err != nil {
		log.Printf("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	log.Printf("Found book to update: %s (ID: %d)", book.Name, book.ID)

	// Parse request body into a map
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		log.Printf("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
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

	if stock, ok := updateData["stock"].(float64); ok && stock >= 0 {
		updates["stock"] = int(stock)
	}

	if categoryID, ok := updateData["category_id"].(float64); ok && categoryID > 0 {
		// Verify category exists
		var category models.Category
		if err := config.DB.First(&category, uint(categoryID)).Error; err != nil {
			utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
			return
		}
		updates["category_id"] = uint(categoryID)
	}

	if genreID, ok := updateData["genre_id"].(float64); ok && genreID > 0 {
		// Verify genre exists
		var genre models.Genre
		if err := config.DB.First(&genre, uint(genreID)).Error; err != nil {
			utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
			return
		}
		updates["genre_id"] = uint(genreID)
	}

	if imageURL, ok := updateData["image_url"].(string); ok && imageURL != "" {
		updates["image_url"] = imageURL
	}

	// Handle images array separately
	if images, ok := updateData["images"].([]interface{}); ok {
		// Convert []interface{} to []string
		imageURLs := make([]string, len(images))
		for i, img := range images {
			if url, ok := img.(string); ok {
				imageURLs[i] = url
			}
		}

		// Delete existing BookImages for this book
		if err := config.DB.Where("book_id = ?", book.ID).Delete(&models.BookImage{}).Error; err != nil {
			log.Printf("Failed to delete old BookImages: %v", err)
			utils.InternalServerError(c, "Failed to update images", err.Error())
			return
		}
		// Insert new BookImages
		for _, url := range imageURLs {
			if url != "" {
				if err := config.DB.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					log.Printf("Failed to insert BookImage: %v", err)
					utils.InternalServerError(c, "Failed to update images", err.Error())
					return
				}
			}
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
		if err := config.DB.Where("isbn = ? AND id != ? AND deleted_at IS NULL", isbn, book.ID).First(&existingBook).Error; err == nil {
			log.Printf("ISBN already exists in an active book: %s", isbn)
			utils.BadRequest(c, "A book with this ISBN already exists", nil)
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

	// Update the book with only the provided fields
	if len(updates) > 0 {
		log.Printf("Updating book with fields: %+v", updates)
		if err := config.DB.Model(&book).Updates(updates).Error; err != nil {
			log.Printf("Failed to update book: %v", err)
			utils.InternalServerError(c, "Failed to update book", err.Error())
			return
		}
	}

	// Fetch the updated book with related data
	var updatedBook models.Book
	if err := config.DB.Preload("Category").Preload("Genre").Preload("BookImages").First(&updatedBook, book.ID).Error; err != nil {
		log.Printf("Failed to fetch updated book: %v", err)
		utils.Success(c, "Book updated, but failed to fetch updated details", gin.H{
			"book": book,
		})
		return
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
		"created_at":       updatedBook.CreatedAt,
		"updated_at":       updatedBook.UpdatedAt,
		"category": gin.H{
			"id":          updatedBook.Category.ID,
			"name":        updatedBook.Category.Name,
			"description": updatedBook.Category.Description,
		},
		"genre": gin.H{
			"id":          updatedBook.Genre.ID,
			"name":        updatedBook.Genre.Name,
			"description": updatedBook.Genre.Description,
		},
	}

	// Add images to response
	imageUrls := make([]string, 0)
	for _, img := range updatedBook.BookImages {
		imageUrls = append(imageUrls, img.URL)
	}
	response["images"] = imageUrls

	log.Printf("Book updated successfully: %s", updatedBook.Name)
	utils.Success(c, "Book updated successfully", gin.H{
		"book": response,
	})
}
