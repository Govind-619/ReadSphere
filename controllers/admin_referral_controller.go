package controllers

import (
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GenerateMixedToken generates a 6-character token with 3 numbers and 3 letters in random order
func GenerateMixedToken() string {
	// Initialize random seed
	rand.Seed(time.Now().UnixNano())

	// Generate 3 random numbers
	numbers := make([]string, 3)
	for i := 0; i < 3; i++ {
		numbers[i] = string(rune('0' + rand.Intn(10)))
	}

	// Generate 3 random uppercase letters
	letters := make([]string, 3)
	for i := 0; i < 3; i++ {
		letters[i] = string(rune('A' + rand.Intn(26)))
	}

	// Combine numbers and letters
	combined := append(numbers, letters...)

	// Shuffle the combined slice
	rand.Shuffle(len(combined), func(i, j int) {
		combined[i], combined[j] = combined[j], combined[i]
	})

	return strings.Join(combined, "")
}

// Admin: Generate referral token URL and coupon for a user
func GenerateReferralToken(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	referralCode := GenerateMixedToken()

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", tx.Error.Error())
		return
	}

	// Create coupon first
	coupon := models.Coupon{
		Code:          "REF-" + referralCode,
		Type:          "percent",
		Value:         10, // 10% referral discount
		MinOrderValue: 100,
		MaxDiscount:   200,
		Expiry:        time.Now().AddDate(0, 1, 0),
		UsageLimit:    1,
		Active:        true,
	}
	if err := tx.Create(&coupon).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create coupon", err.Error())
		return
	}

	// Then create referral with the coupon ID
	referral := models.Referral{
		ReferrerUserID: req.UserID,
		ReferralCode:   referralCode,
		ReferralToken:  referralCode,
		CouponID:       coupon.ID,
	}
	if err := tx.Create(&referral).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create referral", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	utils.Success(c, "Referral token generated successfully", gin.H{
		"referral_token": referralCode,
		"referral_url":   "/referral/" + referralCode,
		"coupon_code":    coupon.Code,
	})
}

// Admin: Get all referrals
func GetAllReferrals(c *gin.Context) {
	var referrals []models.Referral
	db := config.DB
	if err := db.Preload("Coupon").Find(&referrals).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch referrals", err.Error())
		return
	}
	utils.Success(c, "Referrals retrieved successfully", gin.H{
		"referrals": referrals,
	})
}

// Admin: Get all user referral codes
func GetAllUserReferralCodes(c *gin.Context) {
	utils.LogInfo("GetAllUserReferralCodes called")

	// Get pagination parameters
	page, _ := utils.GetPaginationParams(c)
	limit := 20
	offset := (page - 1) * limit

	// Get all user referral codes with user details
	var userCodes []models.UserReferralCode
	var total int64

	// Count total
	if err := config.DB.Model(&models.UserReferralCode{}).Count(&total).Error; err != nil {
		utils.LogError("Failed to count user referral codes: %v", err)
		utils.InternalServerError(c, "Failed to count referral codes", err.Error())
		return
	}

	// Get referral codes with pagination
	if err := config.DB.Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&userCodes).Error; err != nil {
		utils.LogError("Failed to get user referral codes: %v", err)
		utils.InternalServerError(c, "Failed to get referral codes", err.Error())
		return
	}

	// Format response with referral statistics
	formattedCodes := make([]gin.H, len(userCodes))
	for i, userCode := range userCodes {
		// Get referral count for this user
		var referralCount int64
		if err := config.DB.Model(&models.ReferralUsage{}).Where("referrer_id = ?", userCode.UserID).Count(&referralCount).Error; err != nil {
			referralCount = 0
		}

		formattedCodes[i] = gin.H{
			"id": userCode.ID,
			"user": gin.H{
				"id":         userCode.User.ID,
				"username":   userCode.User.Username,
				"email":      userCode.User.Email,
				"first_name": userCode.User.FirstName,
				"last_name":  userCode.User.LastName,
			},
			"referral_code":   userCode.ReferralCode,
			"referral_url":    "/referral/" + userCode.ReferralCode,
			"total_referrals": referralCount,
			"is_active":       userCode.IsActive,
			"created_at":      userCode.CreatedAt.Format("2006-01-02 15:04:05"),
		}
	}

	utils.LogInfo("Successfully retrieved %d user referral codes", len(userCodes))
	utils.SuccessWithPagination(c, "User referral codes retrieved successfully", gin.H{
		"referral_codes": formattedCodes,
	}, total, page, limit)
}

// Admin: Get detailed referral statistics for a specific user
func GetUserReferralStats(c *gin.Context) {
	utils.LogInfo("GetUserReferralStats called")

	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		utils.BadRequest(c, "Invalid user ID", nil)
		return
	}

	// Get user referral code
	var userCode models.UserReferralCode
	if err := config.DB.Preload("User").Where("user_id = ?", userID).First(&userCode).Error; err != nil {
		utils.NotFound(c, "User referral code not found")
		return
	}

	// Get all referrals by this user
	var referrals []models.ReferralUsage
	if err := config.DB.Preload("ReferredUser").
		Preload("ReferrerCoupon").
		Preload("ReferredCoupon").
		Where("referrer_id = ?", userID).
		Order("created_at DESC").
		Find(&referrals).Error; err != nil {
		utils.InternalServerError(c, "Failed to get referral details", err.Error())
		return
	}

	// Format referral details
	formattedReferrals := make([]gin.H, len(referrals))
	for i, referral := range referrals {
		formattedReferrals[i] = gin.H{
			"id": referral.ID,
			"referred_user": gin.H{
				"id":         referral.ReferredUser.ID,
				"username":   referral.ReferredUser.Username,
				"email":      referral.ReferredUser.Email,
				"first_name": referral.ReferredUser.FirstName,
				"last_name":  referral.ReferredUser.LastName,
			},
			"used_at": referral.UsedAt.Format("2006-01-02 15:04:05"),
			"referrer_coupon": gin.H{
				"id":       referral.ReferrerCouponID,
				"code":     referral.ReferrerCoupon.Code,
				"discount": referral.ReferrerCoupon.Value,
				"type":     referral.ReferrerCoupon.Type,
				"expiry":   referral.ReferrerCoupon.Expiry.Format("2006-01-02"),
			},
			"referred_coupon": gin.H{
				"id":       referral.ReferredCouponID,
				"code":     referral.ReferredCoupon.Code,
				"discount": referral.ReferredCoupon.Value,
				"type":     referral.ReferredCoupon.Type,
				"expiry":   referral.ReferredCoupon.Expiry.Format("2006-01-02"),
			},
		}
	}

	utils.Success(c, "User referral statistics retrieved successfully", gin.H{
		"user": gin.H{
			"id":         userCode.User.ID,
			"username":   userCode.User.Username,
			"email":      userCode.User.Email,
			"first_name": userCode.User.FirstName,
			"last_name":  userCode.User.LastName,
		},
		"referral_code":   userCode.ReferralCode,
		"referral_url":    "/referral/" + userCode.ReferralCode,
		"total_referrals": len(referrals),
		"is_active":       userCode.IsActive,
		"created_at":      userCode.CreatedAt.Format("2006-01-02 15:04:05"),
		"referrals":       formattedReferrals,
	})
}

// Admin: Toggle referral code status (activate/deactivate)
func ToggleReferralCodeStatus(c *gin.Context) {
	utils.LogInfo("ToggleReferralCodeStatus called")

	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Find user referral code
	var userCode models.UserReferralCode
	if err := config.DB.Where("user_id = ?", req.UserID).First(&userCode).Error; err != nil {
		utils.NotFound(c, "User referral code not found")
		return
	}

	// Toggle status
	userCode.IsActive = !userCode.IsActive

	if err := config.DB.Save(&userCode).Error; err != nil {
		utils.InternalServerError(c, "Failed to update referral code status", err.Error())
		return
	}

	status := "activated"
	if !userCode.IsActive {
		status = "deactivated"
	}

	utils.Success(c, "Referral code status updated successfully", gin.H{
		"user_id":       req.UserID,
		"referral_code": userCode.ReferralCode,
		"status":        status,
		"is_active":     userCode.IsActive,
	})
}

// Admin: Get overall referral statistics
func GetReferralStatistics(c *gin.Context) {
	utils.LogInfo("GetReferralStatistics called")

	// Total users with referral codes
	var totalUsersWithCodes int64
	if err := config.DB.Model(&models.UserReferralCode{}).Count(&totalUsersWithCodes).Error; err != nil {
		utils.InternalServerError(c, "Failed to count users with referral codes", err.Error())
		return
	}

	// Total active referral codes
	var activeCodes int64
	if err := config.DB.Model(&models.UserReferralCode{}).Where("is_active = ?", true).Count(&activeCodes).Error; err != nil {
		utils.InternalServerError(c, "Failed to count active referral codes", err.Error())
		return
	}

	// Total referrals made
	var totalReferrals int64
	if err := config.DB.Model(&models.ReferralUsage{}).Count(&totalReferrals).Error; err != nil {
		utils.InternalServerError(c, "Failed to count total referrals", err.Error())
		return
	}

	// Top referrers (users with most referrals)
	type TopReferrer struct {
		ID            uint   `json:"id"`
		Username      string `json:"username"`
		Email         string `json:"email"`
		FirstName     string `json:"first_name"`
		LastName      string `json:"last_name"`
		ReferralCode  string `json:"referral_code"`
		ReferralCount int64  `json:"referral_count"`
	}

	var topReferrers []TopReferrer
	if err := config.DB.Raw(`
		SELECT 
			u.id, u.username, u.email, u.first_name, u.last_name,
			urc.referral_code,
			COUNT(ru.id) as referral_count
		FROM users u
		JOIN user_referral_codes urc ON u.id = urc.user_id
		LEFT JOIN referral_usages ru ON u.id = ru.referrer_id
		GROUP BY u.id, urc.referral_code
		ORDER BY referral_count DESC
		LIMIT 10
	`).Scan(&topReferrers).Error; err != nil {
		utils.InternalServerError(c, "Failed to get top referrers", err.Error())
		return
	}

	utils.Success(c, "Referral statistics retrieved successfully", gin.H{
		"total_users_with_codes": totalUsersWithCodes,
		"active_referral_codes":  activeCodes,
		"total_referrals_made":   totalReferrals,
		"top_referrers":          topReferrers,
	})
}
