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
