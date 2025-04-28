package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// UploadBookImages handles multiple image uploads for a book
func UploadBookImages(c *gin.Context) {
	bookIDStr := c.Param("id")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil || bookID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
		return
	}
	files := form.File["images[]"]
	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No images uploaded"})
		return
	}

	uploadDir := "uploads/books"
	_ = os.MkdirAll(uploadDir, os.ModePerm)

	var imageRecords []models.BookImage
	for _, file := range files {
		timestamp := time.Now().UnixNano()
		filename := fmt.Sprintf("%d_%s", timestamp, filepath.Base(file.Filename))
		filepath := filepath.Join(uploadDir, filename)
		if err := c.SaveUploadedFile(file, filepath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file", "file": file.Filename})
			return
		}
		imageRecords = append(imageRecords, models.BookImage{
			BookID: uint(bookID),
			URL:    "/" + filepath,
		})
	}
	if err := config.DB.Create(&imageRecords).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save images to DB", "reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Images uploaded", "images": imageRecords})
}

// GetBookImages returns all images for a book
func GetBookImages(c *gin.Context) {
	bookIDStr := c.Param("id")
	bookID, err := strconv.Atoi(bookIDStr)
	if err != nil || bookID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid book ID"})
		return
	}
	var images []models.BookImage
	if err := config.DB.Where("book_id = ?", bookID).Find(&images).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch images", "reason": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"images": images})
}
