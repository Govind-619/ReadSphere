package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UploadBookImages handles multiple image uploads for a book
func UploadBookImages(c *gin.Context) {
	utils.LogInfo("UploadBookImages called")

	// Parse book ID
	bookIDStr := c.Param("id")
	bookID, err := strconv.ParseUint(bookIDStr, 10, 32)
	if err != nil {
		utils.LogError("Invalid book ID: %v", err)
		utils.BadRequest(c, "Invalid book ID", "Please provide a valid book ID")
		return
	}
	utils.LogDebug("Processing upload for book ID: %d", bookID)

	// Check if book exists
	var book models.Book
	if err := config.DB.First(&book, bookID).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	// Get uploaded files
	form, err := c.MultipartForm()
	if err != nil {
		utils.LogError("Failed to parse form: %v", err)
		utils.BadRequest(c, "Invalid form data", "Please provide valid image files")
		return
	}

	files := form.File["images"]
	if len(files) == 0 {
		utils.LogError("No images uploaded")
		utils.BadRequest(c, "No images uploaded", "Please select at least one image to upload")
		return
	}

	if len(files) > 5 {
		utils.LogError("Too many images uploaded: %d", len(files))
		utils.BadRequest(c, "Too many images", "Maximum 5 images allowed per book")
		return
	}
	utils.LogDebug("Processing %d image files", len(files))

	// Create upload directory
	uploadDir := "uploads/books"
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		utils.LogError("Failed to create upload directory: %v", err)
		utils.InternalServerError(c, "Failed to process upload", err.Error())
		return
	}
	utils.LogDebug("Created upload directory: %s", uploadDir)

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process upload", tx.Error.Error())
		return
	}

	var uploadedImages []gin.H
	for _, file := range files {
		// Validate file
		if err := utils.ValidateImageFile(file); err != nil {
			tx.Rollback()
			utils.LogError("Invalid file: %v", err)
			utils.BadRequest(c, "Invalid file", err.Error())
			return
		}

		// Generate unique filename
		timestamp := time.Now().UnixNano()
		filename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(file.Filename))
		filepath := filepath.Join(uploadDir, filename)
		utils.LogDebug("Generated filename: %s", filename)

		// Save file
		if err := c.SaveUploadedFile(file, filepath); err != nil {
			tx.Rollback()
			utils.LogError("Failed to save file: %v", err)
			utils.InternalServerError(c, "Failed to save file", err.Error())
			return
		}

		// Create image record
		imageURL := "/" + filepath
		bookImage := models.BookImage{
			BookID: uint(bookID),
			URL:    imageURL,
		}

		if err := tx.Create(&bookImage).Error; err != nil {
			tx.Rollback()
			utils.LogError("Failed to save image record: %v", err)
			utils.InternalServerError(c, "Failed to save image record", err.Error())
			return
		}

		uploadedImages = append(uploadedImages, gin.H{
			"id":  bookImage.ID,
			"url": imageURL,
		})
		utils.LogDebug("Created image record with ID: %d", bookImage.ID)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to complete upload", err.Error())
		return
	}

	utils.LogInfo("Successfully uploaded %d images for book %d", len(uploadedImages), bookID)
	utils.Success(c, "Images uploaded successfully", gin.H{
		"book_id": bookID,
		"images":  uploadedImages,
		"count":   len(uploadedImages),
	})
}

// GetBookImages returns all images for a book
func GetBookImages(c *gin.Context) {
	utils.LogInfo("GetBookImages called")

	// Parse book ID
	bookIDStr := c.Param("id")
	bookID, err := strconv.ParseUint(bookIDStr, 10, 32)
	if err != nil {
		utils.LogError("Invalid book ID: %v", err)
		utils.BadRequest(c, "Invalid book ID", "Please provide a valid book ID")
		return
	}
	utils.LogDebug("Fetching images for book ID: %d", bookID)

	// Check if book exists
	var book models.Book
	if err := config.DB.First(&book, bookID).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}

	// Get book images
	var images []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&images).Error; err != nil {
		utils.LogError("Failed to fetch images: %v", err)
		utils.InternalServerError(c, "Failed to fetch images", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d images for book %d", len(images), bookID)

	// Format response
	var formattedImages []gin.H
	for _, img := range images {
		formattedImages = append(formattedImages, gin.H{
			"id":  img.ID,
			"url": img.URL,
		})
	}

	utils.LogInfo("Successfully retrieved images for book %d", bookID)
	utils.Success(c, "Images retrieved successfully", gin.H{
		"book_id": bookID,
		"images":  formattedImages,
		"count":   len(formattedImages),
	})
}

// DeleteBookImage deletes a specific image from a book
func DeleteBookImage(c *gin.Context) {
	utils.LogInfo("DeleteBookImage called")

	// Parse image ID
	imageID := c.Param("image_id")
	if imageID == "" {
		utils.LogError("Empty image ID provided")
		utils.BadRequest(c, "Invalid image ID", "Please provide a valid image ID")
		return
	}
	utils.LogDebug("Deleting image with ID: %s", imageID)

	// Get image record
	var image models.BookImage
	if err := config.DB.First(&image, imageID).Error; err != nil {
		utils.LogError("Image not found: %v", err)
		utils.NotFound(c, "Image not found")
		return
	}

	// Delete physical file if it exists
	if image.URL != "" {
		filePath := strings.TrimPrefix(image.URL, "/")
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			utils.LogError("Failed to delete file: %v", err)
			// Continue with database deletion even if file deletion fails
		} else {
			utils.LogDebug("Deleted physical file: %s", filePath)
		}
	}

	// Delete from database
	if err := config.DB.Delete(&image).Error; err != nil {
		utils.LogError("Failed to delete image record: %v", err)
		utils.InternalServerError(c, "Failed to delete image", err.Error())
		return
	}

	utils.LogInfo("Successfully deleted image %d for book %d", image.ID, image.BookID)
	utils.Success(c, "Image deleted successfully", gin.H{
		"book_id":  image.BookID,
		"image_id": image.ID,
	})
}
