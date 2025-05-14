package controllers

import (
	"fmt"
	"math"
	"strconv"
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
	var req CreateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Convert code to uppercase
	req.Code = strings.ToUpper(req.Code)

	// Validate expiry date
	if req.Expiry.Before(time.Now()) {
		utils.BadRequest(c, "Expiry date must be in the future", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Check if coupon code already exists (case-insensitive)
	var existingCoupon models.Coupon
	if err := tx.Where("LOWER(code) = LOWER(?)", req.Code).First(&existingCoupon).Error; err == nil {
		tx.Rollback()
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
			utils.BadRequest(c, "Coupon code already exists", nil)
			return
		}
		utils.InternalServerError(c, "Failed to create coupon", err.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

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
		utils.Unauthorized(c, "User not found")
		return
	}

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Get the coupon
	var coupon models.Coupon
	if err := tx.Where("code = ? AND active = ?", req.Code, true).First(&coupon).Error; err != nil {
		tx.Rollback()
		utils.NotFound(c, "Invalid or inactive coupon")
		return
	}

	// Check if coupon has expired
	if time.Now().After(coupon.Expiry) {
		tx.Rollback()
		utils.BadRequest(c, "Coupon has expired", nil)
		return
	}

	// Check if coupon has reached usage limit
	if coupon.UsedCount >= coupon.UsageLimit {
		tx.Rollback()
		utils.BadRequest(c, "Coupon usage limit reached", nil)
		return
	}

	// Check if user has already used this coupon
	var userCoupon models.UserCoupon
	if err := tx.Where("user_id = ? AND coupon_id = ?", user.(models.User).ID, coupon.ID).First(&userCoupon).Error; err == nil {
		tx.Rollback()
		utils.BadRequest(c, "You have already used this coupon", nil)
		return
	}

	// Calculate cart total before any discounts
	var cartItems []models.Cart
	if err := tx.Where("user_id = ?", user.(models.User).ID).Find(&cartItems).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to fetch cart items", nil)
		return
	}

	var subtotal float64
	var productDiscountTotal float64
	var categoryDiscountTotal float64

	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil {
			continue
		}
		// Calculate product and category discounts
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)
		productDiscountAmount := (book.Price * offerBreakdown.ProductOfferPercent / 100) * float64(item.Quantity)
		categoryDiscountAmount := (book.Price * offerBreakdown.CategoryOfferPercent / 100) * float64(item.Quantity)

		subtotal += book.Price * float64(item.Quantity)
		productDiscountTotal += productDiscountAmount
		categoryDiscountTotal += categoryDiscountAmount
	}

	// Check if cart total meets minimum order value
	if subtotal < coupon.MinOrderValue {
		tx.Rollback()
		utils.BadRequest(c, "Cart total is less than minimum order value for this coupon", nil)
		return
	}

	// Calculate coupon discount
	var couponDiscount float64
	if coupon.Type == "percent" {
		couponDiscount = (subtotal * coupon.Value) / 100
		if couponDiscount > coupon.MaxDiscount {
			couponDiscount = coupon.MaxDiscount
		}
	} else {
		couponDiscount = coupon.Value
	}

	// Delete any existing active coupons for this user
	if err := tx.Where("user_id = ?", user.(models.User).ID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		if tx.Migrator().HasTable(&models.UserActiveCoupon{}) {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to clear previous active coupons", nil)
			return
		}
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
		utils.InternalServerError(c, "Failed to save active coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	// Calculate final total after all discounts
	totalDiscount := productDiscountTotal + categoryDiscountTotal + couponDiscount
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	utils.Success(c, "Coupon applied successfully", gin.H{
		"subtotal":          fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":   fmt.Sprintf("%.2f", math.Round(couponDiscount*100)/100),
		"coupon_code":       coupon.Code,
		"total_discount":    fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":       fmt.Sprintf("%.2f", finalTotal),
	})
}

// RemoveCoupon removes a coupon from the user's cart
func RemoveCoupon(c *gin.Context) {
	// Ensure the UserActiveCoupon table exists
	config.EnsureUserActiveCouponTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	db := config.DB
	// Get the coupon
	var coupon models.Coupon
	if err := db.Where("code = ?", req.Code).First(&coupon).Error; err != nil {
		utils.NotFound(c, "Invalid coupon")
		return
	}

	// Delete active coupon record
	if err := db.Where("user_id = ? AND coupon_id = ?", user.(models.User).ID, coupon.ID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		if db.Migrator().HasTable(&models.UserActiveCoupon{}) {
			utils.InternalServerError(c, "Failed to remove active coupon", nil)
			return
		}
	}

	// Calculate cart totals after removing coupon
	var cartItems []models.Cart
	if err := db.Where("user_id = ?", user.(models.User).ID).Find(&cartItems).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch cart items", nil)
		return
	}

	var subtotal float64
	var productDiscountTotal float64
	var categoryDiscountTotal float64

	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil {
			continue
		}
		// Calculate product and category discounts
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(book.ID, book.CategoryID)
		productDiscountAmount := (book.Price * offerBreakdown.ProductOfferPercent / 100) * float64(item.Quantity)
		categoryDiscountAmount := (book.Price * offerBreakdown.CategoryOfferPercent / 100) * float64(item.Quantity)

		subtotal += book.Price * float64(item.Quantity)
		productDiscountTotal += productDiscountAmount
		categoryDiscountTotal += categoryDiscountAmount
	}

	// Calculate final total after all discounts (excluding coupon)
	totalDiscount := productDiscountTotal + categoryDiscountTotal
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	utils.Success(c, "Coupon removed successfully", gin.H{
		"subtotal":          fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":   "0.00",
		"coupon_code":       "",
		"total_discount":    fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":       fmt.Sprintf("%.2f", finalTotal),
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
		utils.InternalServerError(c, "Failed to count coupons", nil)
		return
	}

	// Apply pagination
	offset := (page - 1) * limit
	var coupons []models.Coupon
	if err := query.Offset(offset).Limit(limit).Find(&coupons).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch coupons", nil)
		return
	}

	// Format coupons with only necessary information
	var formattedCoupons []gin.H
	for _, coupon := range coupons {
		isExpired := time.Now().After(coupon.Expiry)
		isValid := coupon.Active && !isExpired

		// Create user-friendly description
		description := fmt.Sprintf("%s %s off on orders above ₹%.2f (Max discount: ₹%.2f)",
			func() string {
				if coupon.Type == "percent" {
					return fmt.Sprintf("%.0f%%", coupon.Value)
				}
				return fmt.Sprintf("₹%.2f", coupon.Value)
			}(),
			func() string {
				if coupon.Type == "percent" {
					return ""
				}
				return "flat"
			}(),
			coupon.MinOrderValue,
			coupon.MaxDiscount,
		)

		// Check if admin is in context to determine response format
		_, isAdmin := c.Get("admin")
		if isAdmin {
			// Admin view - show additional details
			formattedCoupons = append(formattedCoupons, gin.H{
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
				"created_at":   coupon.CreatedAt.Format("2006-01-02 15:04:05"),
			})
		} else {
			// User view - show minimal details
			formattedCoupons = append(formattedCoupons, gin.H{
				"code":         strings.ToUpper(coupon.Code),
				"description":  description,
				"expiry":       coupon.Expiry.Format("2006-01-02"),
				"is_valid":     isValid,
				"max_discount": fmt.Sprintf("%.2f", coupon.MaxDiscount),
				"min_order":    fmt.Sprintf("%.2f", coupon.MinOrderValue),
			})
		}
	}

	utils.Success(c, "Coupons retrieved successfully", gin.H{
		"coupons": formattedCoupons,
		"pagination": gin.H{
			"current_page": page,
			"per_page":     limit,
			"total":        total,
			"total_pages":  int(math.Ceil(float64(total) / float64(limit))),
		},
	})
}

// DeleteCoupon deletes an existing coupon
func DeleteCoupon(c *gin.Context) {
	// Check if admin is in context
	admin, exists := c.Get("admin")
	if !exists {
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	_, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	// Get identifier from URL parameter
	identifier := c.Param("id")
	if identifier == "" {
		utils.BadRequest(c, "Coupon identifier is required", nil)
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
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
			utils.NotFound(c, "Coupon not found")
			return
		}
	}

	// Check if coupon has been used
	if coupon.UsedCount > 0 {
		tx.Rollback()
		utils.BadRequest(c, "Cannot delete a coupon that has been used", nil)
		return
	}

	// Delete the coupon
	if err := tx.Delete(&coupon).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to delete coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	utils.Success(c, "Coupon deleted successfully", nil)
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
		utils.Unauthorized(c, "Admin not found in context")
		return
	}

	_, ok := admin.(models.Admin)
	if !ok {
		utils.InternalServerError(c, "Invalid admin type", nil)
		return
	}

	// Get identifier from URL parameter
	identifier := c.Param("id")
	if identifier == "" {
		utils.BadRequest(c, "Coupon identifier is required", nil)
		return
	}

	var req UpdateCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
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
			utils.NotFound(c, "Coupon not found")
			return
		}
	}

	// Validate expiry date if provided
	if !req.Expiry.IsZero() && req.Expiry.Before(time.Now()) {
		tx.Rollback()
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
		utils.InternalServerError(c, "Failed to update coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	// Return concise response
	isExpired := time.Now().After(coupon.Expiry)
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
