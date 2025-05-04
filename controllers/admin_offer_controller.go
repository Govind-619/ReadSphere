package controllers

import (
	"net/http"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use RFC3339."})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offer)
}

func ListProductOffers(c *gin.Context) {
	var offers []models.ProductOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offers)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Offer not found"})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DiscountPercent != 0 {
		offer.DiscountPercent = req.DiscountPercent
	}
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			offer.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			offer.EndDate = t
		}
	}
	if req.Active != nil {
		offer.Active = *req.Active
	}
	if err := config.DB.Save(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offer)
}

func DeleteProductOffer(c *gin.Context) {
	id := c.Param("id")
	result := config.DB.Delete(&models.ProductOffer{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product offer not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Offer deleted"})
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	start, err1 := time.Parse(time.RFC3339, req.StartDate)
	end, err2 := time.Parse(time.RFC3339, req.EndDate)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use RFC3339."})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offer)
}

func ListCategoryOffers(c *gin.Context) {
	var offers []models.CategoryOffer
	if err := config.DB.Find(&offers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offers)
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
		c.JSON(http.StatusNotFound, gin.H{"error": "Offer not found"})
		return
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.DiscountPercent != 0 {
		offer.DiscountPercent = req.DiscountPercent
	}
	if req.StartDate != "" {
		if t, err := time.Parse(time.RFC3339, req.StartDate); err == nil {
			offer.StartDate = t
		}
	}
	if req.EndDate != "" {
		if t, err := time.Parse(time.RFC3339, req.EndDate); err == nil {
			offer.EndDate = t
		}
	}
	if req.Active != nil {
		offer.Active = *req.Active
	}
	if err := config.DB.Save(&offer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, offer)
}

func DeleteCategoryOffer(c *gin.Context) {
	id := c.Param("id")
	result := config.DB.Delete(&models.CategoryOffer{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category offer not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Offer deleted"})
}
