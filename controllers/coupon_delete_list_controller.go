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

// GetCoupons returns all active coupons
func GetCoupons(c *gin.Context) {
	utils.LogInfo("GetCoupons called")

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
	utils.LogInfo("Fetching coupons with pagination - page: %d, limit: %d, sort: %s %s", page, limit, sortBy, order)

	// Build query
	query := config.DB.Model(&models.Coupon{})

	// Apply sorting
	if sortBy != "" {
		query = query.Order(fmt.Sprintf("%s %s", sortBy, order))
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		utils.LogError("Failed to count coupons: %v", err)
		utils.InternalServerError(c, "Failed to count coupons", nil)
		return
	}

	// Apply pagination
	offset := (page - 1) * limit
	var coupons []models.Coupon
	if err := query.Offset(offset).Limit(limit).Find(&coupons).Error; err != nil {
		utils.LogError("Failed to fetch coupons: %v", err)
		utils.InternalServerError(c, "Failed to fetch coupons", nil)
		return
	}
	utils.LogInfo("Retrieved %d coupons out of %d total", len(coupons), total)

	// Format coupons with only necessary information
	var formattedCoupons []gin.H
	_, isAdmin := c.Get("admin")
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

	utils.LogInfo("Successfully formatted %d coupons for %s view", len(formattedCoupons), func() string {
		if isAdmin {
			return "admin"
		}
		return "user"
	}())
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
	utils.LogInfo("DeleteCoupon called")

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
	utils.LogInfo("Processing deletion for coupon identifier: %s", identifier)

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

	// Check if coupon has been used
	if coupon.UsedCount > 0 {
		tx.Rollback()
		utils.LogError("Cannot delete coupon %s: it has been used %d times", coupon.Code, coupon.UsedCount)
		utils.BadRequest(c, "Cannot delete a coupon that has been used", nil)
		return
	}

	// Delete the coupon
	if err := tx.Delete(&coupon).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to delete coupon %s: %v", coupon.Code, err)
		utils.InternalServerError(c, "Failed to delete coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction: %v", err)
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	utils.LogInfo("Successfully deleted coupon with ID: %d, code: %s", coupon.ID, coupon.Code)
	utils.Success(c, "Coupon deleted successfully", nil)
}
