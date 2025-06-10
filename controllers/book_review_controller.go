package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetBookReviews handles fetching reviews for a book
func GetBookReviews(c *gin.Context) {
	utils.LogInfo("GetBookReviews called")

	bookID := c.Param("id")
	utils.LogDebug("Fetching reviews for book ID: %s", bookID)

	var reviews []models.Review
	if err := config.DB.Preload("User").Where("book_id = ?", bookID).Find(&reviews).Error; err != nil {
		utils.LogError("Failed to fetch reviews: %v", err)
		utils.InternalServerError(c, "Failed to fetch reviews", err.Error())
		return
	}

	utils.LogDebug("Found %d reviews for book ID: %s", len(reviews), bookID)
	utils.LogInfo("Successfully retrieved reviews for book ID: %s", bookID)
	utils.Success(c, "Reviews retrieved successfully", reviews)
}

// ApproveReview handles approving a book review
func ApproveReview(c *gin.Context) {
	utils.LogInfo("ApproveReview called")

	reviewID := c.Param("reviewId")
	utils.LogDebug("Approving review ID: %s", reviewID)

	var review models.Review
	if err := config.DB.First(&review, reviewID).Error; err != nil {
		utils.LogError("Review not found: %v", err)
		utils.NotFound(c, "Review not found")
		return
	}
	utils.LogDebug("Found review to approve for book ID: %d", review.BookID)

	review.IsApproved = true
	if err := config.DB.Save(&review).Error; err != nil {
		utils.LogError("Failed to approve review: %v", err)
		utils.InternalServerError(c, "Failed to approve review", err.Error())
		return
	}

	utils.LogInfo("Successfully approved review ID: %s", reviewID)
	utils.Success(c, "Review approved successfully", review)
}

// DeleteReview handles deleting a book review
func DeleteReview(c *gin.Context) {
	utils.LogInfo("DeleteReview called")

	reviewID := c.Param("reviewId")
	utils.LogDebug("Deleting review ID: %s", reviewID)

	var review models.Review
	if err := config.DB.First(&review, reviewID).Error; err != nil {
		utils.LogError("Review not found: %v", err)
		utils.NotFound(c, "Review not found")
		return
	}
	utils.LogDebug("Found review to delete for book ID: %d", review.BookID)

	if err := config.DB.Delete(&review).Error; err != nil {
		utils.LogError("Failed to delete review: %v", err)
		utils.InternalServerError(c, "Failed to delete review", err.Error())
		return
	}

	utils.LogInfo("Successfully deleted review ID: %s", reviewID)
	utils.Success(c, "Review deleted successfully", gin.H{"message": "Review deleted successfully"})
}

// GetBookDetails retrieves details of a specific book
