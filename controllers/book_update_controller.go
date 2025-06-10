package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UpdateBook handles book updates
func UpdateBook(c *gin.Context) {
	utils.LogInfo("UpdateBook called")

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

	utils.LogInfo("Admin authenticated: %s", adminModel.Email)

	// Get book ID from URL parameter
	bookID := c.Param("id")
	if bookID == "" {
		utils.LogError("Book ID not provided in URL")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	utils.LogInfo("Updating book with ID: %s", bookID)

	// Check if book exists
	var book models.Book
	if err := config.DB.First(&book, bookID).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	utils.LogInfo("Found book: %s (ID: %s)", book.Name, bookID)

	// Parse request body
	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		utils.LogError("Invalid input: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}

	utils.LogInfo("Received update data: %v", updateData)

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process update", nil)
		return
	}

	// Create a map to store fields to update
	updates := make(map[string]interface{})

	// Process each field in the update request
	if name, ok := updateData["name"].(string); ok && name != "" {
		updates["name"] = name
		utils.LogInfo("Updating name to: %s", name)
	}
	if description, ok := updateData["description"].(string); ok && description != "" {
		updates["description"] = description
		utils.LogInfo("Updating description")
	}
	if price, ok := updateData["price"].(float64); ok && price > 0 {
		updates["price"] = price
		utils.LogInfo("Updating price to: %f", price)
	}
	if originalPrice, ok := updateData["original_price"].(float64); ok && originalPrice > 0 {
		updates["original_price"] = originalPrice
		utils.LogInfo("Updating original price to: %f", originalPrice)
	}
	if stock, ok := updateData["stock"].(float64); ok && stock >= 0 {
		updates["stock"] = int(stock)
		utils.LogInfo("Updating stock to: %d", int(stock))
	}
	if categoryID, ok := updateData["category_id"].(float64); ok && categoryID > 0 {
		// Verify category exists
		var category models.Category
		if err := tx.First(&category, uint(categoryID)).Error; err != nil {
			tx.Rollback()
			utils.LogError("Invalid category ID: %v", err)
			utils.BadRequest(c, "Invalid category ID", "The specified category does not exist")
			return
		}
		updates["category_id"] = uint(categoryID)
		utils.LogInfo("Updating category to: %s (ID: %f)", category.Name, categoryID)
	}
	if genreID, ok := updateData["genre_id"].(float64); ok && genreID > 0 {
		// Verify genre exists
		var genre models.Genre
		if err := tx.First(&genre, uint(genreID)).Error; err != nil {
			tx.Rollback()
			utils.LogError("Invalid genre ID: %v", err)
			utils.BadRequest(c, "Invalid genre ID", "The specified genre does not exist")
			return
		}
		updates["genre_id"] = uint(genreID)
		utils.LogInfo("Updating genre to: %s (ID: %f)", genre.Name, genreID)
	}
	if imageURL, ok := updateData["image_url"].(string); ok {
		updates["image_url"] = imageURL
		utils.LogInfo("Updating image URL")
	}
	if isActive, exists := updateData["is_active"].(bool); exists {
		updates["is_active"] = isActive
		utils.LogInfo("Updating is_active to: %v", isActive)
	}
	if isFeatured, exists := updateData["is_featured"].(bool); exists {
		updates["is_featured"] = isFeatured
		utils.LogInfo("Updating is_featured to: %v", isFeatured)
	}
	if blocked, exists := updateData["blocked"].(bool); exists {
		updates["blocked"] = blocked
		utils.LogInfo("Updating blocked status to: %v", blocked)
	}
	if author, ok := updateData["author"].(string); ok && author != "" {
		updates["author"] = author
		utils.LogInfo("Updating author to: %s", author)
	}
	if publisher, ok := updateData["publisher"].(string); ok && publisher != "" {
		updates["publisher"] = publisher
		utils.LogInfo("Updating publisher to: %s", publisher)
	}
	if isbn, ok := updateData["isbn"].(string); ok && isbn != "" {
		// Check ISBN uniqueness
		var existingBook models.Book
		if err := tx.Where("isbn = ? AND id != ?", isbn, bookID).First(&existingBook).Error; err == nil {
			tx.Rollback()
			utils.LogError("ISBN conflict: %s already exists for book ID: %d", isbn, existingBook.ID)
			utils.Conflict(c, "A book with this ISBN already exists", nil)
			return
		}
		updates["isbn"] = isbn
		utils.LogInfo("Updating ISBN to: %s", isbn)
	}
	if publicationYear, ok := updateData["publication_year"].(float64); ok && publicationYear > 0 {
		updates["publication_year"] = int(publicationYear)
		utils.LogInfo("Updating publication year to: %d", int(publicationYear))
	}
	if pages, ok := updateData["pages"].(float64); ok && pages > 0 {
		updates["pages"] = int(pages)
		utils.LogInfo("Updating pages to: %d", int(pages))
	}
	if language, ok := updateData["language"].(string); ok && language != "" {
		updates["language"] = language
		utils.LogInfo("Updating language to: %s", language)
	}
	if format, ok := updateData["format"].(string); ok && format != "" {
		updates["format"] = format
		utils.LogInfo("Updating format to: %s", format)
	}

	// Handle images array if provided
	if images, ok := updateData["images"].([]interface{}); ok {
		utils.LogInfo("Updating images array with %d images", len(images))
		// Delete existing images
		if err := tx.Where("book_id = ?", bookID).Delete(&models.BookImage{}).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to delete existing images: %v", err)
			utils.InternalServerError(c, "Failed to update images", nil)
			return
		}

		// Add new images
		for _, img := range images {
			if url, ok := img.(string); ok && url != "" {
				if err := tx.Create(&models.BookImage{BookID: book.ID, URL: url}).Error; err != nil {
					tx.Rollback()
					utils.LogError("Failed to add new image: %v", err)
					utils.InternalServerError(c, "Failed to add new images", nil)
					return
				}
			}
		}
		utils.LogInfo("Successfully updated images")
	}

	// Update the book if there are changes
	if len(updates) > 0 {
		utils.LogInfo("Applying %d updates to book", len(updates))
		if err := tx.Model(&book).Updates(updates).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to update book: %v", err)
			utils.InternalServerError(c, "Failed to update book", nil)
			return
		}
	} else {
		utils.LogInfo("No updates to apply")
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to complete update", nil)
		return
	}

	utils.LogInfo("Transaction committed successfully")

	// Fetch updated book details
	var updatedBook models.Book
	if err := config.DB.First(&updatedBook, bookID).Error; err != nil {
		utils.LogError("Failed to fetch updated book: %v", err)
		utils.InternalServerError(c, "Book updated but failed to fetch details", nil)
		return
	}

	// Fetch category details
	var category models.Category
	if err := config.DB.First(&category, updatedBook.CategoryID).Error; err != nil {
		utils.LogError("Failed to fetch category: %v", err)
	}

	// Fetch genre details
	var genre models.Genre
	if err := config.DB.First(&genre, updatedBook.GenreID).Error; err != nil {
		utils.LogError("Failed to fetch genre: %v", err)
	}

	// Fetch book images
	var bookImages []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&bookImages).Error; err != nil {
		utils.LogError("Failed to fetch book images: %v", err)
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
		"blocked":          updatedBook.Blocked,
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

	utils.LogInfo("Book updated successfully: %s (ID: %s, Blocked: %v, Active: %v)",
		updatedBook.Name, bookID, updatedBook.Blocked, updatedBook.IsActive)
	utils.Success(c, "Book updated successfully", gin.H{
		"book": response,
	})
}
