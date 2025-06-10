package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// DeleteBook handles book deletion
func DeleteBook(c *gin.Context) {
	utils.LogInfo("DeleteBook called")

	// Get the ID from the URL parameter
	id := c.Param("id")
	if id == "" {
		utils.LogError("Book ID is empty")
		utils.BadRequest(c, "Book ID is required", nil)
		return
	}

	utils.LogDebug("Attempting to delete book with ID: %s", id)

	// Check if the book exists
	var book models.Book
	if err := config.DB.First(&book, id).Error; err != nil {
		utils.LogError("Book not found: %v", err)
		utils.NotFound(c, "Book not found")
		return
	}
	utils.LogDebug("Found book to delete: %s", book.Name)

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to process deletion", nil)
		return
	}
	utils.LogDebug("Started database transaction")

	// Delete associated book images first
	if err := tx.Where("book_id = ?", id).Delete(&models.BookImage{}).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to delete book images: %v", err)
		utils.InternalServerError(c, "Failed to delete book images", nil)
		return
	}
	utils.LogDebug("Deleted associated book images")

	// Delete the book
	if err := tx.Delete(&book).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to delete book: %v", err)
		utils.InternalServerError(c, "Failed to delete book", nil)
		return
	}
	utils.LogDebug("Deleted book record")

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to complete deletion", nil)
		return
	}
	utils.LogDebug("Successfully committed transaction")

	utils.LogInfo("Book deleted successfully: %s (ID: %s)", book.Name, id)
	utils.Success(c, "Book deleted successfully", nil)
}
