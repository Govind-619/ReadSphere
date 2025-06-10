package controllers

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ---- Product Offer CRUD ----
func CreateProductOffer(c *gin.Context) {
	utils.LogInfo("CreateProductOffer called")

	var req struct {
		ProductID       uint    `json:"product_id" binding:"required"`
		DiscountPercent float64 `json:"discount_percent" binding:"required"`
		StartDate       string  `json:"start_date" binding:"required"` // ISO8601
		EndDate         string  `json:"end_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request data: %v", err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogDebug("Received request for product %d with %f%% discount", req.ProductID, req.DiscountPercent)

	// Validate discount percentage
	if req.DiscountPercent <= 0 || req.DiscountPercent > 100 {
		utils.LogError("Invalid discount percentage: %f", req.DiscountPercent)
		utils.BadRequest(c, "Discount percentage must be between 0 and 100", nil)
		return
	}

	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		utils.LogError("Invalid date format: start=%v, end=%v", err1, err2)
		utils.BadRequest(c, "Invalid date format. Use RFC3339.", nil)
		return
	}
	utils.LogDebug("Parsed dates - Start: %s, End: %s", start.Format(time.RFC3339), end.Format(time.RFC3339))

	// Check if end date is in the past
	if end.Before(time.Now()) {
		utils.LogError("End date is in the past: %s", end.Format(time.RFC3339))
		utils.BadRequest(c, "End date cannot be in the past", nil)
		return
	}

	// Check for existing active offers for this product
	var existingOffer models.ProductOffer
	if err := config.DB.Where("product_id = ? AND active = ? AND end_date > ?",
		req.ProductID, true, time.Now()).First(&existingOffer).Error; err == nil {
		utils.LogError("Active offer already exists for product %d", req.ProductID)
		utils.BadRequest(c, "An active offer already exists for this product", nil)
		return
	}
	utils.LogDebug("No active offers found for product %d", req.ProductID)

	offer := models.ProductOffer{
		ProductID:       req.ProductID,
		DiscountPercent: req.DiscountPercent,
		StartDate:       start,
		EndDate:         end,
		Active:          true,
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		utils.LogError("Failed to create offer: %v", err)
		utils.InternalServerError(c, "Failed to create offer", err.Error())
		return
	}
	utils.LogDebug("Created new offer with ID %d", offer.ID)

	utils.LogInfo("Successfully created product offer for product %d", req.ProductID)
	utils.Success(c, "Product offer created successfully", gin.H{
		"offer": gin.H{
			"id":               offer.ID,
			"product_id":       offer.ProductID,
			"discount_percent": offer.DiscountPercent,
			"start_date":       offer.StartDate.Format("2006-01-02"),
			"end_date":         offer.EndDate.Format("2006-01-02"),
			"active":           offer.Active,
			"is_expired":       time.Now().After(offer.EndDate),
		},
	})
}

func ListProductOffers(c *gin.Context) {
	utils.LogInfo("ListProductOffers called")

	var offers []models.ProductOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		utils.LogError("Failed to fetch offers: %v", err)
		utils.InternalServerError(c, "Failed to fetch offers", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d product offers", len(offers))

	var formattedOffers []gin.H
	for _, offer := range offers {
		formattedOffers = append(formattedOffers, gin.H{
			"id":               offer.ID,
			"product_id":       offer.ProductID,
			"discount_percent": offer.DiscountPercent,
			"start_date":       offer.StartDate.Format("2006-01-02"),
			"end_date":         offer.EndDate.Format("2006-01-02"),
			"active":           offer.Active,
			"is_expired":       time.Now().After(offer.EndDate),
		})
	}

	utils.LogInfo("Successfully retrieved %d product offers", len(formattedOffers))
	utils.Success(c, "Product offers retrieved successfully", gin.H{
		"offers": formattedOffers,
	})
}

func UpdateProductOffer(c *gin.Context) {
	utils.LogInfo("UpdateProductOffer called")

	var req struct {
		DiscountPercent float64 `json:"discount_percent"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		Active          *bool   `json:"active"`
	}
	id := c.Param("id")
	utils.LogDebug("Updating offer with ID: %s", id)

	var offer models.ProductOffer
	if err := config.DB.First(&offer, id).Error; err != nil {
		utils.LogError("Offer not found: %v", err)
		utils.NotFound(c, "Offer not found")
		return
	}
	utils.LogDebug("Found existing offer for product %d", offer.ProductID)

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request data: %v", err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Update fields if provided
	if req.DiscountPercent != 0 {
		// Validate discount percentage
		if req.DiscountPercent <= 0 || req.DiscountPercent > 100 {
			utils.LogError("Invalid discount percentage: %f", req.DiscountPercent)
			utils.BadRequest(c, "Discount percentage must be between 0 and 100", nil)
			return
		}
		offer.DiscountPercent = req.DiscountPercent
		utils.LogDebug("Updated discount percentage to %f", req.DiscountPercent)
	}
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			offer.StartDate = t
			utils.LogDebug("Updated start date to %s", t.Format(time.RFC3339))
		} else {
			utils.LogError("Invalid start date format: %v", err)
			utils.BadRequest(c, "Invalid start date format. Use RFC3339.", nil)
			return
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			// Check if end date is in the past
			if t.Before(time.Now()) {
				utils.LogError("End date is in the past: %s", t.Format(time.RFC3339))
				utils.BadRequest(c, "End date cannot be in the past", nil)
				return
			}
			offer.EndDate = t
			utils.LogDebug("Updated end date to %s", t.Format(time.RFC3339))
		} else {
			utils.LogError("Invalid end date format: %v", err)
			utils.BadRequest(c, "Invalid end date format. Use RFC3339.", nil)
			return
		}
	}
	if req.Active != nil {
		offer.Active = *req.Active
		utils.LogDebug("Updated active status to %v", *req.Active)
	}

	if err := config.DB.Save(&offer).Error; err != nil {
		utils.LogError("Failed to update offer: %v", err)
		utils.InternalServerError(c, "Failed to update offer", err.Error())
		return
	}

	utils.LogInfo("Successfully updated product offer %d", offer.ID)
	utils.Success(c, "Product offer updated successfully", gin.H{
		"offer": gin.H{
			"id":               offer.ID,
			"product_id":       offer.ProductID,
			"discount_percent": offer.DiscountPercent,
			"start_date":       offer.StartDate.Format("2006-01-02"),
			"end_date":         offer.EndDate.Format("2006-01-02"),
			"active":           offer.Active,
			"is_expired":       time.Now().After(offer.EndDate),
		},
	})
}

func DeleteProductOffer(c *gin.Context) {
	utils.LogInfo("DeleteProductOffer called")

	id := c.Param("id")
	utils.LogDebug("Deleting offer with ID: %s", id)

	result := config.DB.Delete(&models.ProductOffer{}, id)
	if result.Error != nil {
		utils.LogError("Failed to delete offer: %v", result.Error)
		utils.InternalServerError(c, "Failed to delete offer", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.LogError("Product offer not found: %s", id)
		utils.NotFound(c, "Product offer not found")
		return
	}

	utils.LogInfo("Successfully deleted product offer %s", id)
	utils.Success(c, "Product offer deleted successfully", nil)
}

// ---- Category Offer CRUD ----
func CreateCategoryOffer(c *gin.Context) {
	utils.LogInfo("CreateCategoryOffer called")

	var req struct {
		CategoryID      uint    `json:"category_id" binding:"required"`
		DiscountPercent float64 `json:"discount_percent" binding:"required"`
		StartDate       string  `json:"start_date" binding:"required"`
		EndDate         string  `json:"end_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request data: %v", err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogDebug("Received request for category %d with %f%% discount", req.CategoryID, req.DiscountPercent)

	// Validate discount percentage
	if req.DiscountPercent <= 0 || req.DiscountPercent > 100 {
		utils.LogError("Invalid discount percentage: %f", req.DiscountPercent)
		utils.BadRequest(c, "Discount percentage must be between 0 and 100", nil)
		return
	}

	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		utils.LogError("Invalid date format: start=%v, end=%v", err1, err2)
		utils.BadRequest(c, "Invalid date format. Use RFC3339.", nil)
		return
	}
	utils.LogDebug("Parsed dates - Start: %s, End: %s", start.Format(time.RFC3339), end.Format(time.RFC3339))

	// Check if end date is in the past
	if end.Before(time.Now()) {
		utils.LogError("End date is in the past: %s", end.Format(time.RFC3339))
		utils.BadRequest(c, "End date cannot be in the past", nil)
		return
	}

	// Check for existing active offers for this category
	var existingOffer models.CategoryOffer
	if err := config.DB.Where("category_id = ? AND active = ? AND end_date > ?",
		req.CategoryID, true, time.Now()).First(&existingOffer).Error; err == nil {
		utils.LogError("Active offer already exists for category %d", req.CategoryID)
		utils.BadRequest(c, "An active offer already exists for this category", nil)
		return
	}
	utils.LogDebug("No active offers found for category %d", req.CategoryID)

	offer := models.CategoryOffer{
		CategoryID:      req.CategoryID,
		DiscountPercent: req.DiscountPercent,
		StartDate:       start,
		EndDate:         end,
		Active:          true,
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		utils.LogError("Failed to create offer: %v", err)
		utils.InternalServerError(c, "Failed to create offer", err.Error())
		return
	}
	utils.LogDebug("Created new offer with ID %d", offer.ID)

	utils.LogInfo("Successfully created category offer for category %d", req.CategoryID)
	utils.Success(c, "Category offer created successfully", gin.H{
		"offer": gin.H{
			"id":               offer.ID,
			"category_id":      offer.CategoryID,
			"discount_percent": offer.DiscountPercent,
			"start_date":       offer.StartDate.Format("2006-01-02"),
			"end_date":         offer.EndDate.Format("2006-01-02"),
			"active":           offer.Active,
			"is_expired":       time.Now().After(offer.EndDate),
		},
	})
}
