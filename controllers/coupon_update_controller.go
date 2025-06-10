package controllers

import (
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UpdateCouponRequest represents the request body for updating an existing coupon
type UpdateCouponRequest struct {
	Type          string    `json:"type" binding:"omitempty,oneof=flat percent"`
	Value         float64   `json:"value" binding:"omitempty,gt=0"`
	MinOrderValue float64   `json:"min_order_value" binding:"omitempty,gt=0"`
	MaxDiscount   float64   `json:"max_discount" binding:"omitempty,gt=0"`
	Expiry        time.Time `json:"expiry" binding:"omitempty"`
	UsageLimit    int       `json:"usage_limit" binding:"omitempty,gt=0"`
	Active        *bool     `json:"active" binding:"omitempty"`
}

// UpdateCoupon updates an existing coupon
func UpdateCoupon(c *gin.Context) {
	utils.LogInfo("UpdateCoupon called")

	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.LogError("Admin not found in context")
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	_, ok := admin.(models.Admin)
	if !ok {
		utils.LogError("Invalid admin type in context")
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	// Get identifier from URL parameter
	identifier := c.Param("id")
	if identifier == "" {
		utils.LogError("Missing coupon identifier")
		utils.BadRequest(c, "Coupon identifier is required", nil)
		return
	}
	utils.LogInfo("Processing update for coupon identifier: %s", identifier)

	var req UpdateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for coupon %s: %v", identifier, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Try to find coupon by ID first
	var coupon models.Coupon
	query := tx.Where("id = ?", identifier)

	// If not found by ID, try by code
	if err := query.First(&coupon).Error; err != nil {
		query = tx.Where("LOWER(code) = LOWER(?)", identifier)
		if err := query.First(&coupon).Error; err != nil {
			tx.Rollback()
			utils.LogError("Coupon not found with identifier: %s", identifier)
			utils.NotFound(c, "Coupon not found")
			return
		}
	}
	utils.LogInfo("Found coupon with ID: %d, code: %s", coupon.ID, coupon.Code)

	// Validate expiry date if provided
	if !req.Expiry.IsZero() && req.Expiry.Before(time.Now()) {
		tx.Rollback()
		utils.LogError("Invalid expiry date for coupon %s: date is in the past", coupon.Code)
		utils.BadRequest(c, "Expiry date must be in the future", nil)
		return
	}

	// Update fields if provided
	updates := make(map[string]interface{})
	if req.Type != "" {
		updates["type"] = req.Type
	}
	if req.Value > 0 {
		updates["value"] = req.Value
	}
	if req.MinOrderValue > 0 {
		updates["min_order_value"] = req.MinOrderValue
	}
	if req.MaxDiscount > 0 {
		updates["max_discount"] = req.MaxDiscount
	}
	if !req.Expiry.IsZero() {
		updates["expiry"] = req.Expiry
	}
	if req.UsageLimit > 0 {
		updates["usage_limit"] = req.UsageLimit
	}
	if req.Active != nil {
		updates["active"] = *req.Active
	}
	updates["updated_at"] = time.Now()

	// Update the coupon
	if err := tx.Model(&coupon).Updates(updates).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update coupon %s: %v", coupon.Code, err)
		utils.InternalServerError(c, "Failed to update coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	// Return concise response
	isExpired := time.Now().After(coupon.Expiry)
	utils.LogInfo("Successfully updated coupon with ID: %d, code: %s", coupon.ID, coupon.Code)
	utils.Success(c, "Coupon updated successfully", gin.H{
		"id":           coupon.ID,
		"code":         strings.ToUpper(coupon.Code),
		"type":         coupon.Type,
		"value":        coupon.Value,
		"min_order":    coupon.MinOrderValue,
		"max_discount": coupon.MaxDiscount,
		"usage_limit":  coupon.UsageLimit,
		"used_count":   coupon.UsedCount,
		"active":       coupon.Active,
		"is_expired":   isExpired,
		"expiry":       coupon.Expiry.Format("2006-01-02"),
		"last_updated": coupon.UpdatedAt.Format("2006-01-02 15:04:05"),
	})
}
