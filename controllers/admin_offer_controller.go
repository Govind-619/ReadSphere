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
	var req struct {
		ProductID       uint    `json:"product_id" binding:"required"`
		DiscountPercent float64 `json:"discount_percent" binding:"required"`
		StartDate       string  `json:"start_date" binding:"required"` // ISO8601
		EndDate         string  `json:"end_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		utils.BadRequest(c, "Invalid date format. Use RFC3339.", nil)
		return
	}

	offer := models.ProductOffer{
		ProductID:       req.ProductID,
		DiscountPercent: req.DiscountPercent,
		StartDate:       start,
		EndDate:         end,
		Active:          true,
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		utils.InternalServerError(c, "Failed to create offer", err.Error())
		return
	}

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
	var offers []models.ProductOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch offers", err.Error())
		return
	}

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

	utils.Success(c, "Product offers retrieved successfully", gin.H{
		"offers": formattedOffers,
	})
}

func UpdateProductOffer(c *gin.Context) {
	var req struct {
		DiscountPercent float64 `json:"discount_percent"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		Active          *bool   `json:"active"`
	}
	id := c.Param("id")
	var offer models.ProductOffer
	if err := config.DB.First(&offer, id).Error; err != nil {
		utils.NotFound(c, "Offer not found")
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Update fields if provided
	if req.DiscountPercent != 0 {
		offer.DiscountPercent = req.DiscountPercent
	}
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			offer.StartDate = t
		} else {
			utils.BadRequest(c, "Invalid start date format. Use RFC3339.", nil)
			return
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			offer.EndDate = t
		} else {
			utils.BadRequest(c, "Invalid end date format. Use RFC3339.", nil)
			return
		}
	}
	if req.Active != nil {
		offer.Active = *req.Active
	}

	if err := config.DB.Save(&offer).Error; err != nil {
		utils.InternalServerError(c, "Failed to update offer", err.Error())
		return
	}

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
	id := c.Param("id")
	result := config.DB.Delete(&models.ProductOffer{}, id)
	if result.Error != nil {
		utils.InternalServerError(c, "Failed to delete offer", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Product offer not found")
		return
	}
	utils.Success(c, "Product offer deleted successfully", nil)
}

// ---- Category Offer CRUD ----
func CreateCategoryOffer(c *gin.Context) {
	var req struct {
		CategoryID      uint    `json:"category_id" binding:"required"`
		DiscountPercent float64 `json:"discount_percent" binding:"required"`
		StartDate       string  `json:"start_date" binding:"required"`
		EndDate         string  `json:"end_date" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		utils.BadRequest(c, "Invalid date format. Use RFC3339.", nil)
		return
	}

	offer := models.CategoryOffer{
		CategoryID:      req.CategoryID,
		DiscountPercent: req.DiscountPercent,
		StartDate:       start,
		EndDate:         end,
		Active:          true,
	}

	if err := config.DB.Create(&offer).Error; err != nil {
		utils.InternalServerError(c, "Failed to create offer", err.Error())
		return
	}

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

func ListCategoryOffers(c *gin.Context) {
	var offers []models.CategoryOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch offers", err.Error())
		return
	}

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

	utils.Success(c, "Category offers retrieved successfully", gin.H{
		"offers": formattedOffers,
	})
}

func UpdateCategoryOffer(c *gin.Context) {
	var req struct {
		DiscountPercent float64 `json:"discount_percent"`
		StartDate       string  `json:"start_date"`
		EndDate         string  `json:"end_date"`
		Active          *bool   `json:"active"`
	}
	id := c.Param("id")
	var offer models.CategoryOffer
	if err := config.DB.First(&offer, id).Error; err != nil {
		utils.NotFound(c, "Offer not found")
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Update fields if provided
	if req.DiscountPercent != 0 {
		offer.DiscountPercent = req.DiscountPercent
	}
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			offer.StartDate = t
		} else {
			utils.BadRequest(c, "Invalid start date format. Use RFC3339.", nil)
			return
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			offer.EndDate = t
		} else {
			utils.BadRequest(c, "Invalid end date format. Use RFC3339.", nil)
			return
		}
	}
	if req.Active != nil {
		offer.Active = *req.Active
	}

	if err := config.DB.Save(&offer).Error; err != nil {
		utils.InternalServerError(c, "Failed to update offer", err.Error())
		return
	}

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
	id := c.Param("id")
	result := config.DB.Delete(&models.CategoryOffer{}, id)
	if result.Error != nil {
		utils.InternalServerError(c, "Failed to delete offer", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Category offer not found")
		return
	}
	utils.Success(c, "Category offer deleted successfully", nil)
}
