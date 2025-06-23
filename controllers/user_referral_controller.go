package controllers

import (
	"math/rand"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GenerateUserReferralCode generates a unique referral code for a user
func GenerateUserReferralCode(userID uint) (string, error) {
	utils.LogInfo("GenerateUserReferralCode called for userID: %d", userID)
	// Generate a 6-character code with 3 letters and 3 numbers
	rand.Seed(time.Now().UnixNano())

	// Generate 3 random letters
	letters := make([]string, 3)
	for i := 0; i < 3; i++ {
		letters[i] = string(rune('A' + rand.Intn(26)))
	}

	// Generate 3 random numbers
	numbers := make([]string, 3)
	for i := 0; i < 3; i++ {
		numbers[i] = string(rune('0' + rand.Intn(10)))
	}

	// Combine and shuffle
	combined := append(letters, numbers...)
	rand.Shuffle(len(combined), func(i, j int) {
		combined[i], combined[j] = combined[j], combined[i]
	})

	return strings.Join(combined, ""), nil
}

// GetOrCreateUserReferralCode gets or creates a permanent referral code for the user
func GetOrCreateUserReferralCode(userID uint) (*models.UserReferralCode, error) {
	utils.LogInfo("GetOrCreateUserReferralCode called for userID: %d", userID)
	var userCode models.UserReferralCode

	// Try to find existing referral code
	err := config.DB.Where("user_id = ?", userID).First(&userCode).Error
	if err == nil {
		// Found existing code
		return &userCode, nil
	}

	// No existing code found, create new one
	referralCode, err := GenerateUserReferralCode(userID)
	if err != nil {
		return nil, err
	}

	// Ensure code is unique
	for {
		var existingCode models.UserReferralCode
		if err := config.DB.Where("referral_code = ?", referralCode).First(&existingCode).Error; err != nil {
			// Code is unique, break
			break
		}
		// Code exists, generate new one
		referralCode, err = GenerateUserReferralCode(userID)
		if err != nil {
			return nil, err
		}
	}

	// Create new referral code
	userCode = models.UserReferralCode{
		UserID:       userID,
		ReferralCode: referralCode,
		IsActive:     true,
	}

	if err := config.DB.Create(&userCode).Error; err != nil {
		return nil, err
	}

	return &userCode, nil
}

// GetUserReferralCode returns the user's permanent referral code
func GetUserReferralCode(c *gin.Context) {
	utils.LogInfo("GetUserReferralCode called")

	// Get user from context
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}

	utils.LogInfo("Getting referral code for user ID: %d", user.ID)

	// Get or create referral code
	userCode, err := GetOrCreateUserReferralCode(user.ID)
	if err != nil {
		utils.LogError("Failed to get/create referral code for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get referral code", err.Error())
		return
	}

	// Get referral statistics
	var referralCount int64
	if err := config.DB.Model(&models.ReferralUsage{}).Where("referrer_id = ?", user.ID).Count(&referralCount).Error; err != nil {
		utils.LogError("Failed to count referrals for user ID: %d: %v", user.ID, err)
		referralCount = 0 // Set to 0 if error
	}

	utils.LogInfo("Successfully retrieved referral code for user ID: %d", user.ID)
	utils.Success(c, "Referral code retrieved successfully", gin.H{
		"data": gin.H{
			"referral_code":   userCode.ReferralCode,
			"referral_url":    "/referral/" + userCode.ReferralCode,
			"total_referrals": referralCount,
			"is_active":       userCode.IsActive,
			"created_at":      userCode.CreatedAt.Format("2006-01-02 15:04:05"),
		},
	})
}

// GetUserReferrals returns the list of people who joined through user's referral
func GetUserReferrals(c *gin.Context) {
	utils.LogInfo("GetUserReferrals called")

	// Get user from context
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}

	utils.LogInfo("Getting referrals for user ID: %d", user.ID)

	// Get pagination parameters
	page, _ := utils.GetPaginationParams(c)
	limit := 10
	offset := (page - 1) * limit

	// Get referral usages with referred user details
	var referrals []models.ReferralUsage
	var total int64

	// Count total
	if err := config.DB.Model(&models.ReferralUsage{}).Where("referrer_id = ?", user.ID).Count(&total).Error; err != nil {
		utils.LogError("Failed to count referrals for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to count referrals", err.Error())
		return
	}

	// Get referrals with pagination
	if err := config.DB.Preload("ReferredUser").
		Preload("ReferrerCoupon").
		Where("referrer_id = ?", user.ID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&referrals).Error; err != nil {
		utils.LogError("Failed to get referrals for user ID: %d: %v", user.ID, err)
		utils.InternalServerError(c, "Failed to get referrals", err.Error())
		return
	}

	// Format response
	formattedReferrals := make([]gin.H, len(referrals))
	for i, referral := range referrals {
		formattedReferrals[i] = gin.H{
			"referred_user": gin.H{
				"id":         referral.ReferredUser.ID,
				"username":   referral.ReferredUser.Username,
				"email":      referral.ReferredUser.Email,
				"first_name": referral.ReferredUser.FirstName,
				"last_name":  referral.ReferredUser.LastName,
			},
			"referrer_coupon_code": referral.ReferrerCoupon.Code,
		}
	}

	utils.LogInfo("Successfully retrieved %d referrals for user ID: %d", len(referrals), user.ID)
	utils.SuccessWithPagination(c, "Referrals retrieved successfully", gin.H{
		"referrals": formattedReferrals,
	}, total, page, limit)
}

// UseReferralCode processes when someone uses a referral code during signup
func UseReferralCode(referralCode string, referredUserID uint) error {
	utils.LogInfo("UseReferralCode called - Code: %s, User ID: %d", referralCode, referredUserID)

	// Find the referral code
	var userCode models.UserReferralCode
	if err := config.DB.Where("referral_code = ? AND is_active = ?", referralCode, true).First(&userCode).Error; err != nil {
		utils.LogError("Referral code not found or inactive: %s", referralCode)
		return err
	}

	// Check if user is trying to use their own referral code
	if userCode.UserID == referredUserID {
		utils.LogError("User trying to use their own referral code: %s", referralCode)
		return utils.NewError("Cannot use your own referral code")
	}

	// Check if this user has already used a referral code
	var existingUsage models.ReferralUsage
	if err := config.DB.Where("referred_user_id = ?", referredUserID).First(&existingUsage).Error; err == nil {
		utils.LogError("User has already used a referral code: %d", referredUserID)
		return utils.NewError("You have already used a referral code")
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Create coupon for referrer (25% off)
	referrerCoupon := models.Coupon{
		Code:          "REF-" + userCode.ReferralCode + "-" + "BONUS",
		Type:          "percent",
		Value:         25,
		MinOrderValue: 100,
		MaxDiscount:   500,
		Expiry:        time.Now().AddDate(0, 1, 0), // 1 month
		UsageLimit:    1,
		Active:        true,
	}

	if err := tx.Create(&referrerCoupon).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create coupon for referred user (25% off)
	referredCoupon := models.Coupon{
		Code:          "REF-NEW-" + userCode.ReferralCode,
		Type:          "percent",
		Value:         25,
		MinOrderValue: 100,
		MaxDiscount:   500,
		Expiry:        time.Now().AddDate(0, 1, 0), // 1 month
		UsageLimit:    1,
		Active:        true,
	}

	if err := tx.Create(&referredCoupon).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create referral usage record
	referralUsage := models.ReferralUsage{
		ReferrerID:       userCode.UserID,
		ReferredUserID:   referredUserID,
		UsedAt:           time.Now(),
		ReferrerCouponID: referrerCoupon.ID,
		ReferredCouponID: referredCoupon.ID,
	}

	if err := tx.Create(&referralUsage).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	utils.LogInfo("Successfully processed referral code usage - Code: %s, Referrer: %d, Referred: %d",
		referralCode, userCode.UserID, referredUserID)

	return nil
}

// GetReferralCodeInfo returns information about a referral code (public endpoint)
func GetReferralCodeInfo(c *gin.Context) {
	utils.LogInfo("GetReferralCodeInfo called")

	referralCode := c.Param("code")
	if referralCode == "" {
		utils.BadRequest(c, "Referral code is required", nil)
		return
	}

	utils.LogInfo("Getting info for referral code: %s", referralCode)

	// Find the referral code
	var userCode models.UserReferralCode
	if err := config.DB.Preload("User").Where("referral_code = ? AND is_active = ?", referralCode, true).First(&userCode).Error; err != nil {
		utils.LogError("Referral code not found or inactive: %s", referralCode)
		utils.NotFound(c, "Invalid or inactive referral code")
		return
	}

	utils.LogInfo("Successfully retrieved referral code info: %s", referralCode)
	utils.Success(c, "Referral code information", gin.H{
		"referral_code": userCode.ReferralCode,
		"referrer": gin.H{
			"username":   userCode.User.Username,
			"first_name": userCode.User.FirstName,
			"last_name":  userCode.User.LastName,
		},
		"benefits": gin.H{
			"referrer_discount": 25,
			"referred_discount": 25,
			"discount_type":     "percent",
			"min_order":         100,
			"max_discount":      500,
			"validity":          "1 month",
		},
		"message": "Use this code during signup to get 25% off on your first order!",
	})
}
