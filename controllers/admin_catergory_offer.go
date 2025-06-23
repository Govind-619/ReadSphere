package controllers

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

func ListCategoryOffers(c *gin.Context) {
	utils.LogInfo("ListCategoryOffers called")

	var offers []models.CategoryOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		utils.LogError("Failed to fetch offers: %v", err)
		utils.InternalServerError(c, "Failed to fetch offers", err.Error())
		return
	}
	utils.LogDebug("Retrieved %d category offers", len(offers))

	var formattedOffers []gin.H
	for _, offer := range offers {
		formattedOffers = append(formattedOffers, gin.H{
			"id":               offer.ID,
			"category_id":      offer.CategoryID,
			"discount_percent": offer.DiscountPercent,
			"start_date":       offer.StartDate.Format("2006-01-02"),
			"end_date":         offer.EndDate.Format("2006-01-02"),
			"active":           offer.Active,
			"is_expired":       time.Now().After(offer.EndDate),
		})
	}

	utils.LogInfo("Successfully retrieved %d category offers", len(formattedOffers))
	utils.Success(c, "Category offers retrieved successfully", gin.H{
		"offers": formattedOffers,
	})
}

func UpdateCategoryOffer(c *gin.Context) {
	utils.LogInfo("UpdateCategoryOffer called")

	var req struct {
		DiscountPercent float64 `json:"discount_percent"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		Active          *bool   `json:"active"`
	}
	id := c.Param("id")
	utils.LogDebug("Updating category offer with ID: %s", id)

	var offer models.CategoryOffer
	if err := config.DB.First(&offer, id).Error; err != nil {
		utils.LogError("Offer not found: %v", err)
		utils.NotFound(c, "Offer not found")
		return
	}
	utils.LogDebug("Found existing offer for category %d", offer.CategoryID)

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request data: %v", err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Validate discount_percent
	if req.DiscountPercent > 100 {
		utils.LogError("Discount percent cannot exceed 100: %f", req.DiscountPercent)
		utils.BadRequest(c, "Discount percent cannot exceed 100", nil)
		return
	}

	// Update fields if provided
	if req.DiscountPercent != 0 {
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

	utils.LogInfo("Successfully updated category offer %d", offer.ID)
	utils.Success(c, "Category offer updated successfully", gin.H{
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

func DeleteCategoryOffer(c *gin.Context) {
	utils.LogInfo("DeleteCategoryOffer called")

	id := c.Param("id")
	utils.LogDebug("Deleting category offer with ID: %s", id)

	result := config.DB.Delete(&models.CategoryOffer{}, id)
	if result.Error != nil {
		utils.LogError("Failed to delete offer: %v", result.Error)
		utils.InternalServerError(c, "Failed to delete offer", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.LogError("Category offer not found: %s", id)
		utils.NotFound(c, "Category offer not found")
		return
	}

	utils.LogInfo("Successfully deleted category offer %s", id)
	utils.Success(c, "Category offer deleted successfully", nil)
}
