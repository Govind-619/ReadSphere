package controllers

import (
	"fmt"
	"math"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ApplyCouponRequest represents the request body for applying a coupon
type ApplyCouponRequest struct {
	Code string `json:"code" binding:"required"`
}

// ApplyCoupon applies a coupon to the user's cart
func ApplyCoupon(c *gin.Context) {
	utils.LogInfo("ApplyCoupon called")

	// Ensure the UserActiveCoupon table exists
	config.EnsureUserActiveCouponTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	userID := user.(models.User).ID
	utils.LogInfo("Processing coupon application for user ID: %d", userID)

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Attempting to apply coupon code: %s for user ID: %d", req.Code, userID)

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction for user ID: %d: %v", userID, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Get the coupon
	var coupon models.Coupon
	if err := tx.Where("code = ? AND active = ?", req.Code, true).First(&coupon).Error; err != nil {
		tx.Rollback()
		utils.LogError("Invalid or inactive coupon code: %s for user ID: %d", req.Code, userID)
		utils.NotFound(c, "Invalid or inactive coupon")
		return
	}

	// Check if coupon has expired
	if time.Now().After(coupon.Expiry) {
		tx.Rollback()
		utils.LogError("Expired coupon code: %s for user ID: %d", req.Code, userID)
		utils.BadRequest(c, "Coupon has expired", nil)
		return
	}

	// Check if coupon has reached usage limit
	if coupon.UsedCount >= coupon.UsageLimit {
		tx.Rollback()
		utils.LogError("Coupon usage limit reached for code: %s, user ID: %d", req.Code, userID)
		utils.BadRequest(c, "Coupon usage limit reached", nil)
		return
	}

	// Check if user has already used this coupon
	var userCoupon models.UserCoupon
	if err := tx.Where("user_id = ? AND coupon_id = ?", userID, coupon.ID).First(&userCoupon).Error; err == nil {
		tx.Rollback()
		utils.LogError("User ID: %d has already used coupon code: %s", userID, req.Code)
		utils.BadRequest(c, "You have already used this coupon", nil)
		return
	}

	// Calculate cart total before any discounts
	var cartItems []models.Cart
	if err := tx.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to fetch cart items for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to fetch cart items", nil)
		return
	}
	utils.LogInfo("Found %d items in cart for user ID: %d", len(cartItems), userID)

	var subtotal float64
	var productDiscountTotal float64
	var categoryDiscountTotal float64
	var totalQuantity int

	// First pass: Calculate subtotal and total quantity
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
		totalQuantity += item.Quantity
	}

	// Check if cart total meets minimum order value
	if subtotal < coupon.MinOrderValue {
		tx.Rollback()
		utils.LogError("Cart total %.2f is less than minimum order value %.2f for coupon: %s, user ID: %d", subtotal, coupon.MinOrderValue, req.Code, userID)
		utils.BadRequest(c, "Cart total is less than minimum order value for this coupon", nil)
		return
	}

	// Calculate total coupon discount
	var totalCouponDiscount float64
	if coupon.Type == "percent" {
		totalCouponDiscount = (subtotal * coupon.Value) / 100
		if totalCouponDiscount > coupon.MaxDiscount {
			totalCouponDiscount = coupon.MaxDiscount
		}
	} else {
		totalCouponDiscount = coupon.Value
	}

	// Calculate discount per quantity unit
	discountPerUnit := totalCouponDiscount / float64(totalQuantity)

	// Delete any existing active coupons for this user
	if err := tx.Where("user_id = ?", userID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		if tx.Migrator().HasTable(&models.UserActiveCoupon{}) {
			tx.Rollback()
			utils.LogError("Failed to clear previous active coupons for user ID: %d: %v", userID, err)
			utils.InternalServerError(c, "Failed to clear previous active coupons", nil)
			return
		}
	}

	// Create active coupon record
	activeUserCoupon := models.UserActiveCoupon{
		UserID:    userID,
		CouponID:  coupon.ID,
		Code:      coupon.Code,
		AppliedAt: time.Now(),
	}

	if err := tx.Create(&activeUserCoupon).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to save active coupon for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to save active coupon", nil)
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to commit transaction", nil)
		return
	}

	// Calculate final total after all discounts
	totalDiscount := productDiscountTotal + categoryDiscountTotal + totalCouponDiscount
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	utils.LogInfo("Successfully applied coupon code: %s for user ID: %d, final total: %.2f", req.Code, userID, finalTotal)
	utils.Success(c, "Coupon applied successfully", gin.H{
		"subtotal":          fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":   fmt.Sprintf("%.2f", math.Round(totalCouponDiscount*100)/100),
		"coupon_code":       coupon.Code,
		"total_discount":    fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":       fmt.Sprintf("%.2f", finalTotal),
		"discount_per_unit": fmt.Sprintf("%.2f", math.Round(discountPerUnit*100)/100),
	})
}

// RemoveCoupon removes a coupon from the user's cart
func RemoveCoupon(c *gin.Context) {
	utils.LogInfo("RemoveCoupon called")

	// Ensure the UserActiveCoupon table exists
	config.EnsureUserActiveCouponTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	userID := user.(models.User).ID
	utils.LogInfo("Processing coupon removal for user ID: %d", userID)

	var req ApplyCouponRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Attempting to remove coupon code: %s for user ID: %d", req.Code, userID)

	db := config.DB
	// Get the coupon
	var coupon models.Coupon
	if err := db.Where("code = ?", req.Code).First(&coupon).Error; err != nil {
		utils.LogError("Invalid coupon code: %s for user ID: %d", req.Code, userID)
		utils.NotFound(c, "Invalid coupon")
		return
	}

	// Delete active coupon record
	if err := db.Where("user_id = ? AND coupon_id = ?", userID, coupon.ID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
		if db.Migrator().HasTable(&models.UserActiveCoupon{}) {
			utils.LogError("Failed to remove active coupon for user ID: %d: %v", userID, err)
			utils.InternalServerError(c, "Failed to remove active coupon", nil)
			return
		}
	}

	// Calculate cart totals after removing coupon
	var cartItems []models.Cart
	if err := db.Where("user_id = ?", userID).Find(&cartItems).Error; err != nil {
		utils.LogError("Failed to fetch cart items for user ID: %d: %v", userID, err)
		utils.InternalServerError(c, "Failed to fetch cart items", nil)
		return
	}
	utils.LogInfo("Found %d items in cart for user ID: %d", len(cartItems), userID)

	var subtotal float64
	var productDiscountTotal float64
	var categoryDiscountTotal float64
	var totalQuantity int

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
		totalQuantity += item.Quantity
	}

	// Calculate final total after all discounts (excluding coupon)
	totalDiscount := productDiscountTotal + categoryDiscountTotal
	finalTotal := math.Round((subtotal-totalDiscount)*100) / 100

	utils.LogInfo("Successfully removed coupon code: %s for user ID: %d, final total: %.2f", req.Code, userID, finalTotal)
	utils.Success(c, "Coupon removed successfully", gin.H{
		"subtotal":          fmt.Sprintf("%.2f", math.Round(subtotal*100)/100),
		"product_discount":  fmt.Sprintf("%.2f", math.Round(productDiscountTotal*100)/100),
		"category_discount": fmt.Sprintf("%.2f", math.Round(categoryDiscountTotal*100)/100),
		"coupon_discount":   "0.00",
		"coupon_code":       "",
		"total_discount":    fmt.Sprintf("%.2f", math.Round(totalDiscount*100)/100),
		"final_total":       fmt.Sprintf("%.2f", finalTotal),
		"discount_per_unit": "0.00",
	})
}
