package controllers

import (
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// CreateCouponRequest represents the request body for creating a new coupon
type CreateCouponRequest struct {
	Code          string    `json:"code" binding:"required"`
	Type          string    `json:"type" binding:"required,oneof=flat percent"`
	Value         float64   `json:"value" binding:"required,gt=0"`
	MinOrderValue float64   `json:"min_order_value" binding:"required,gt=0"`
	MaxDiscount   float64   `json:"max_discount" binding:"required,gt=0"`
	Expiry        time.Time `json:"expiry" binding:"required"`
	UsageLimit    int       `json:"usage_limit" binding:"required,gt=0"`
}

// CreateCoupon creates a new coupon
func CreateCoupon(c *gin.Context) {
	utils.LogInfo("CreateCoupon called")

	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Processing coupon creation with code: %s", req.Code)

	// Enforce percentage coupon value limit
	if err := utils.ValidateCouponValue(req.Type, req.Value); err != nil {
		utils.LogError("Invalid coupon value for code %s: %v", req.Code, err)
		utils.BadRequest(c, err.Error(), nil)
		return
	}
	// Convert code to uppercase
	req.Code = strings.ToUpper(req.Code)

	// Validate expiry date
	if req.Expiry.Before(time.Now()) {
		utils.LogError("Invalid expiry date for coupon code %s: date is in the past", req.Code)
		utils.BadRequest(c, "Expiry date must be in the future", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction: %v", tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Check if coupon code already exists (case-insensitive)
	var existingCoupon models.Coupon
	if err := tx.Where("LOWER(code) = LOWER(?)", req.Code).First(&existingCoupon).Error; err == nil {
		tx.Rollback()
		utils.LogError("Coupon code already exists: %s", req.Code)
		utils.BadRequest(c, "Coupon code already exists", nil)
		return
	}

	coupon := models.Coupon{
		Code:          req.Code,
		Type:          req.Type,
		Value:         req.Value,
		MinOrderValue: req.MinOrderValue,
		MaxDiscount:   req.MaxDiscount,
		Expiry:        req.Expiry,
		UsageLimit:    req.UsageLimit,
		Active:        true,
	}

	if err := tx.Create(&coupon).Error; err != nil {
		tx.Rollback()
		// Check if error is due to unique constraint violation
		if err.Error() == "ERROR: duplicate key value violates unique constraint \"idx_coupons_code_lower\" (SQLSTATE 23505)" {
			utils.LogError("Duplicate coupon code: %s", req.Code)
			utils.BadRequest(c, "Coupon code already exists", nil)
			return
		}
		utils.LogError("Failed to create coupon: %v", err)
		utils.InternalServerError(c, "Failed to create coupon", err.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	utils.LogInfo("Successfully created coupon with code: %s, ID: %d", coupon.Code, coupon.ID)
	// Return consistent response format
	utils.Success(c, "Coupon created successfully", gin.H{
		"id":           coupon.ID,
		"code":         strings.ToUpper(coupon.Code),
		"type":         coupon.Type,
		"value":        coupon.Value,
		"min_order":    coupon.MinOrderValue,
		"max_discount": coupon.MaxDiscount,
		"usage_limit":  coupon.UsageLimit,
		"used_count":   0,
		"active":       coupon.Active,
		"is_expired":   false,
		"expiry":       coupon.Expiry.Format("2006-01-02"),
		"created_at":   coupon.CreatedAt.Format("2006-01-02 15:04:05"),
	})
}
