package controllers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// UpdateBookByField handles book updates by any unique field
func UpdateBookByField(c *gin.Context) {
	log.Printf("UpdateBookByField called")

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

	// Get field name and value from URL parameters
	fieldName := c.Param("field")
	fieldValue := c.Param("value")

	if fieldName == "" || fieldValue == "" {
		log.Printf("Field name or value not provided in URL")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Field name and value are required"})
		return
	}

	log.Printf("Updating book with %s: %s", fieldName, fieldValue)

	// Check if book exists - use a raw SQL query to avoid the scanning error with the images column
	var book models.Book
	query := `
		SELECT 
			id, created_at, updated_at, deleted_at, 
			name, description, price, stock, category_id, 
			image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages
		FROM books 
		WHERE ` + fieldName + ` = ? AND deleted_at IS NULL
	`

	if err := config.DB.Raw(query, fieldValue).Scan(&book).Error; err != nil {
		log.Printf("Book not found: %v", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}

	log.Printf("Found book to update: %s (ID: %d)", book.Name, book.ID)

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
		// Convert price from USD to INR (using approximate conversion rate of 1 USD = 83 INR)
		updates["price"] = price * 83
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
		if err := config.DB.Exec("UPDATE books SET images = "+imagesArray+" WHERE id = ?", book.ID).Error; err != nil {
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
		if err := config.DB.Where("isbn = ? AND id != ? AND deleted_at IS NULL", isbn, book.ID).First(&existingBook).Error; err == nil {
			log.Printf("ISBN already exists in an active book: %s", isbn)
			c.JSON(http.StatusBadRequest, gin.H{"error": "A book with this ISBN already exists"})
			return
		}
		updates["isbn"] = isbn
	}

	if publicationYear, ok := updateData["publication_year"].(float64); ok && publicationYear > 0 {
		updates["publication_year"] = int(publicationYear)
	}

	if genre, ok := updateData["genre"].(string); ok && genre != "" {
		updates["genre"] = genre
	}

	if pages, ok := updateData["pages"].(float64); ok && pages > 0 {
		updates["pages"] = int(pages)
	}

	// Update the book with only the provided fields (excluding images which we already updated)
	if len(updates) > 0 {
		log.Printf("Updating book with fields: %+v", updates)

		// Use a raw SQL query to update the book with the provided fields
		updateQuery := "UPDATE books SET "
		updateParams := []interface{}{}

		// Build the SET clause
		setClauses := []string{}
		for field, value := range updates {
			setClauses = append(setClauses, fmt.Sprintf("%s = ?", field))
			updateParams = append(updateParams, value)
		}

		// Add updated_at timestamp
		setClauses = append(setClauses, "updated_at = ?")
		updateParams = append(updateParams, time.Now())

		updateQuery += strings.Join(setClauses, ", ")

		// Add WHERE clause with ID
		updateQuery += " WHERE id = ?"
		updateParams = append(updateParams, book.ID)

		// Execute the update query
		if err := config.DB.Exec(updateQuery, updateParams...).Error; err != nil {
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
			name, description, price, stock, category_id, 
			image_url, is_active, is_featured, views, 
			average_rating, total_reviews, author, publisher, 
			isbn, publication_year, genre, pages
		FROM books 
		WHERE id = ?
	`

	if err := config.DB.Raw(query, book.ID).Scan(&updatedBook).Error; err != nil {
		log.Printf("Failed to fetch updated book: %v", err)
		// Return the book we updated, even if we couldn't fetch the updated version
		c.JSON(http.StatusOK, gin.H{
			"message": "Book updated successfully, but failed to fetch updated version",
			"book":    book,
			"note":    "Price is in Indian Rupees (INR)",
		})
		return
	}

	// Now fetch the images separately
	var images []string
	if err := config.DB.Raw("SELECT images FROM books WHERE id = ?", book.ID).Scan(&images).Error; err != nil {
		log.Printf("Failed to fetch images for book %d: %v", book.ID, err)
		// Continue anyway, as we have the book data
	} else {
		updatedBook.Images = images
	}

	log.Printf("Book updated successfully: %s", updatedBook.Name)
	c.JSON(http.StatusOK, gin.H{
		"message": "Book updated successfully",
		"book":    updatedBook,
		"note":    "Price is in Indian Rupees (INR)",
	})
}
