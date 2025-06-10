package controllers

import (
	"fmt"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UpdateBookByField handles book updates by any unique field
func UpdateBookByField(c *gin.Context) {
	utils.LogInfo("UpdateBookByField called")

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

	// Get field name and value from URL parameters
	fieldName := c.Param("field")
	fieldValue := c.Param("value")

	if fieldName == "" || fieldValue == "" {
		utils.LogError("Field name or value not provided in URL")
		utils.BadRequest(c, "Field name and value are required", nil)
		return
	}

	utils.LogDebug("Updating book with %s: %s", fieldName, fieldValue)

	// Check if book exists
	var book models.Book
	if err := config.DB.Where(fmt.Sprintf("%s = ?", fieldName), fieldValue).First(&book).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	utils.LogDebug("Found book to update: %s (ID: %d)", book.Name, book.ID)

	// Parse request body into a map
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	utils.LogDebug("Update data received: %+v", updateData)

	// Create a map to store fields to update
	updates := make(map[string]interface{})

	// Check each field in the request and update only those that are provided
	if name, ok := updateData["name"].(string); ok && name != "" {
		updates["name"] = name
		utils.LogDebug("Updating name to: %s", name)
	}

	if description, ok := updateData["description"].(string); ok && description != "" {
		updates["description"] = description
		utils.LogDebug("Updating description")
	}

	if price, ok := updateData["price"].(float64); ok && price > 0 {
		updates["price"] = price
		utils.LogDebug("Updating price to: %.2f", price)
	}

	if stock, ok := updateData["stock"].(float64); ok && stock >= 0 {
		updates["stock"] = int(stock)
		utils.LogDebug("Updating stock to: %d", int(stock))
	}

	if categoryID, ok := updateData["category_id"].(float64); ok && categoryID > 0 {
		// Verify category exists
		var category models.Category
		if err := config.DB.First(&category, uint(categoryID)).Error; err != nil {
			utils.LogError("Category not found: %v", err)
			utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
			return
		}
		updates["category_id"] = uint(categoryID)
		utils.LogDebug("Updating category to: %s", category.Name)
	}

	if genreID, ok := updateData["genre_id"].(float64); ok && genreID > 0 {
		// Verify genre exists
		var genre models.Genre
		if err := config.DB.First(&genre, uint(genreID)).Error; err != nil {
			utils.LogError("Genre not found: %v", err)
			utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
			return
		}
		updates["genre_id"] = uint(genreID)
		utils.LogDebug("Updating genre to: %s", genre.Name)
	}

	if imageURL, ok := updateData["image_url"].(string); ok && imageURL != "" {
		updates["image_url"] = imageURL
		utils.LogDebug("Updating image URL")
	}

	// Handle images array separately
	if images, ok := updateData["images"].([]interface{}); ok {
		utils.LogDebug("Processing %d images", len(images))
		// Convert []interface{} to []string
		imageURLs := make([]string, len(images))
		for i, img := range images {
			if url, ok := img.(string); ok {
				imageURLs[i] = url
			}
		}

		// Delete existing BookImages for this book
		if err := config.DB.Where("book_id = ?", book.ID).Delete(&models.BookImage{}).Error; err != nil {
			utils.LogError("Failed to delete old BookImages: %v", err)
			utils.InternalServerError(c, "Failed to update images", err.Error())
			return
		}
		utils.LogDebug("Deleted existing book images")

		// Insert new BookImages
		for _, url := range imageURLs {
			if url != "" {
				if err := config.DB.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					utils.LogError("Failed to insert BookImage: %v", err)
					utils.InternalServerError(c, "Failed to update images", err.Error())
					return
				}
			}
		}
		utils.LogDebug("Added %d new book images", len(imageURLs))
	}

	if isActive, ok := updateData["is_active"].(bool); ok {
		updates["is_active"] = isActive
		utils.LogDebug("Updating is_active to: %v", isActive)
	}

	if isFeatured, ok := updateData["is_featured"].(bool); ok {
		updates["is_featured"] = isFeatured
		utils.LogDebug("Updating is_featured to: %v", isFeatured)
	}

	if author, ok := updateData["author"].(string); ok && author != "" {
		updates["author"] = author
		utils.LogDebug("Updating author to: %s", author)
	}

	if publisher, ok := updateData["publisher"].(string); ok && publisher != "" {
		updates["publisher"] = publisher
		utils.LogDebug("Updating publisher to: %s", publisher)
	}

	if isbn, ok := updateData["isbn"].(string); ok && isbn != "" {
		// Check if ISBN already exists in an active book (excluding the current book)
		var existingBook models.Book
		if err := config.DB.Where("isbn = ? AND id != ? AND deleted_at IS NULL", isbn, book.ID).First(&existingBook).Error; err == nil {
			utils.LogError("ISBN already exists in an active book: %s", isbn)
			utils.BadRequest(c, "A book with this ISBN already exists", nil)
			return
		}
		updates["isbn"] = isbn
		utils.LogDebug("Updating ISBN to: %s", isbn)
	}

	if publicationYear, ok := updateData["publication_year"].(float64); ok && publicationYear > 0 {
		updates["publication_year"] = int(publicationYear)
		utils.LogDebug("Updating publication year to: %d", int(publicationYear))
	}

	if pages, ok := updateData["pages"].(float64); ok && pages > 0 {
		updates["pages"] = int(pages)
		utils.LogDebug("Updating pages to: %d", int(pages))
	}

	if language, ok := updateData["language"].(string); ok && language != "" {
		updates["language"] = language
		utils.LogDebug("Updating language to: %s", language)
	}

	if format, ok := updateData["format"].(string); ok && format != "" {
		updates["format"] = format
		utils.LogDebug("Updating format to: %s", format)
	}

	// Update the book with only the provided fields
	if len(updates) > 0 {
		utils.LogDebug("Updating book with %d fields", len(updates))
		if err := config.DB.Model(&book).Updates(updates).Error; err != nil {
			utils.LogError("Failed to update book: %v", err)
			utils.InternalServerError(c, "Failed to update book", err.Error())
			return
		}
		utils.LogDebug("Successfully updated book fields")
	}

	// Fetch the updated book with related data
	var updatedBook models.Book
	if err := config.DB.Preload("Category").Preload("Genre").Preload("BookImages").First(&updatedBook, book.ID).Error; err != nil {
		utils.LogError("Failed to fetch updated book: %v", err)
		utils.Success(c, "Book updated, but failed to fetch updated details", gin.H{
			"book": book,
		})
		return
	}
	utils.LogDebug("Successfully fetched updated book details")

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
	utils.LogDebug("Prepared response data with %d images", len(imageUrls))

	utils.LogInfo("Book updated successfully: %s", updatedBook.Name)
	utils.Success(c, "Book updated successfully", gin.H{
		"book": response,
	})
}
