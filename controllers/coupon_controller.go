package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
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
	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert code to uppercase
	req.Code = strings.ToUpper(req.Code)

	// Validate expiry date
	if req.Expiry.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expiry date must be in the future"})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	// Check if coupon code already exists (case-insensitive)
	var existingCoupon models.Coupon
	if err := tx.Where("LOWER(code) = LOWER(?)", req.Code).First(&existingCoupon).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon code already exists"})
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
			c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon code already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create coupon: " + err.Error()})
		return
	}

	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Coupon created successfully",
		"coupon":  coupon,
	})
}

// ApplyCouponRequest represents the request body for applying a coupon
type ApplyCouponRequest struct {
	Code string `json:"code" binding:"required"`
}

// ApplyCoupon applies a coupon to the user's cart
func ApplyCoupon(c *gin.Context) {
	// Ensure the UserActiveCoupon table exists
	config.EnsureUserActiveCouponTableExists()

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	// Get the coupon
	var coupon models.Coupon
	if err := tx.Where("code = ? AND active = ?", req.Code, true).First(&coupon).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid or inactive coupon"})
		return
	}

	// Check if coupon has expired
	if time.Now().After(coupon.Expiry) {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon has expired"})
		return
	}

	// Check if coupon has reached usage limit
	if coupon.UsedCount >= coupon.UsageLimit {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon usage limit reached"})
		return
	}

	// Check if user has already used this coupon
	var userCoupon models.UserCoupon
	if err := tx.Where("user_id = ? AND coupon_id = ?", user.(models.User).ID, coupon.ID).First(&userCoupon).Error; err == nil {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "You have already used this coupon"})
		return
	}

	// Get user's cart total
	var cartTotal float64
	if err := tx.Model(&models.Cart{}).
		Select("COALESCE(SUM(books.price * carts.quantity), 0)").
		Joins("JOIN books ON books.id = carts.book_id").
		Where("carts.user_id = ?", user.(models.User).ID).
		Scan(&cartTotal).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to calculate cart total"})
		return
	}

	// Check if cart total meets minimum order value
	if cartTotal < coupon.MinOrderValue {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart total is less than minimum order value for this coupon"})
		return
	}

	// Calculate discount
	var discount float64
	if coupon.Type == "percent" {
		discount = (cartTotal * coupon.Value) / 100
		if discount > coupon.MaxDiscount {
			discount = coupon.MaxDiscount
		}
	} else {
		discount = coupon.Value
	}

	// Delete any existing active coupons for this user
	if err := tx.Where("user_id = ?", user.(models.User).ID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		// Don't fail if the error is related to table not existing yet - just log and continue
		if tx.Migrator().HasTable(&models.UserActiveCoupon{}) {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear previous active coupons"})
			return
		}
		// If table doesn't exist, just continue with the process
	}

	// Create active coupon record
	activeUserCoupon := models.UserActiveCoupon{
		UserID:    user.(models.User).ID,
		CouponID:  coupon.ID,
		Code:      coupon.Code,
		AppliedAt: time.Now(),
	}

	if err := tx.Create(&activeUserCoupon).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save active coupon"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Coupon applied successfully",
		"discount":    discount,
		"final_total": cartTotal - discount,
	})
}

// RemoveCoupon removes a coupon from the user's cart
func RemoveCoupon(c *gin.Context) {
	// Ensure the UserActiveCoupon table exists
	config.EnsureUserActiveCouponTableExists()

	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the coupon
	var coupon models.Coupon
	if err := config.DB.Where("code = ?", req.Code).First(&coupon).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid coupon"})
		return
	}

	// Delete active coupon record
	if err := config.DB.Where("user_id = ? AND coupon_id = ?", user.(models.User).ID, coupon.ID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		// Don't fail if the error is related to table not existing yet
		if config.DB.Migrator().HasTable(&models.UserActiveCoupon{}) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove active coupon"})
			return
		}
		// If table doesn't exist, just continue
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Coupon removed successfully",
	})
}

// GetCoupons returns all active coupons
func GetCoupons(c *gin.Context) {
	// Get pagination parameters
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	sortBy := c.DefaultQuery("sort_by", "created_at")
	order := c.DefaultQuery("order", "desc")

	// Validate pagination parameters
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}

	// Build query
	query := config.DB.Model(&models.Coupon{})

	// Apply sorting
	if sortBy != "" {
		query = query.Order(fmt.Sprintf("%s %s", sortBy, order))
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count coupons"})
		return
	}

	// Apply pagination
	offset := (page - 1) * limit
	var coupons []models.Coupon
	if err := query.Offset(offset).Limit(limit).Find(&coupons).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch coupons"})
		return
	}

	// Convert all codes to uppercase for consistency
	for i := range coupons {
		coupons[i].Code = strings.ToUpper(coupons[i].Code)
	}

	c.JSON(http.StatusOK, gin.H{
		"coupons": coupons,
		"pagination": gin.H{
			"total":   total,
			"page":    page,
			"limit":   limit,
			"pages":   int(math.Ceil(float64(total) / float64(limit))),
			"sort_by": sortBy,
			"order":   order,
		},
	})
}

// DeleteCoupon deletes an existing coupon
func DeleteCoupon(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found in context"})
		return
	}

	_, ok := admin.(models.Admin)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid admin type"})
		return
	}

	// Get identifier from URL parameter
	identifier := c.Param("id")
	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon identifier is required"})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Coupon not found"})
			return
		}
	}

	// Check if coupon has been used
	if coupon.UsedCount > 0 {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot delete a coupon that has been used"})
		return
	}

	// Delete the coupon
	if err := tx.Delete(&coupon).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete coupon"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Coupon deleted successfully",
	})
}

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
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found in context"})
		return
	}

	_, ok := admin.(models.Admin)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid admin type"})
		return
	}

	// Get identifier from URL parameter
	identifier := c.Param("id")
	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Coupon identifier is required"})
		return
	}

	var req UpdateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
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
			c.JSON(http.StatusNotFound, gin.H{"error": "Coupon not found"})
			return
		}
	}

	// Validate expiry date if provided
	if !req.Expiry.IsZero() && req.Expiry.Before(time.Now()) {
		tx.Rollback()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Expiry date must be in the future"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update coupon"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Coupon updated successfully",
		"coupon":  coupon,
	})
}
