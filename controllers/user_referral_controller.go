package controllers

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetReferralToken returns the referral token details
func GetReferralToken(c *gin.Context) {
	utils.LogInfo("GetReferralToken called")
	token := c.Param("token")
	utils.LogDebug("Processing referral token: %s", token)

	var referral models.Referral
	db := config.DB
	if err := db.Preload("Coupon").Where("referral_token = ?", token).First(&referral).Error; err != nil {
		utils.LogError("Invalid referral token: %s: %v", token, err)
		utils.NotFound(c, "Invalid referral token")
		return
	}
	utils.LogDebug("Successfully retrieved referral details - Token: %s, Code: %s", token, referral.ReferralCode)

	utils.LogInfo("Successfully retrieved referral token details for token: %s", token)
	utils.Success(c, "Referral token details retrieved", gin.H{
		"referral_token": referral.ReferralToken,
		"coupon_code":    referral.Coupon.Code,
		"discount":       referral.Coupon.Value,
		"min_order":      referral.Coupon.MinOrderValue,
		"max_discount":   referral.Coupon.MaxDiscount,
		"expiry":         referral.Coupon.Expiry.Format("2006-01-02"),
	})
}

// AcceptReferralToken accepts a referral via token URL (signup)
func AcceptReferralToken(c *gin.Context) {
	utils.LogInfo("AcceptReferralToken called")
	token := c.Param("token")
	utils.LogDebug("Processing referral token acceptance for token: %s", token)

	var referral models.Referral
	db := config.DB
	if err := db.Where("referral_token = ?", token).First(&referral).Error; err != nil {
		utils.LogError("Invalid referral token: %s: %v", token, err)
		utils.NotFound(c, "Invalid referral token")
		return
	}
	utils.LogDebug("Found referral record - Token: %s, Code: %s", token, referral.ReferralCode)

	// Check if user is authenticated
	userVal, exists := c.Get("user")
	if !exists || userVal == nil {
		utils.LogError("User not authenticated for referral token: %s", token)
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	user := userVal.(models.User)
	utils.LogDebug("User authenticated - User ID: %d", user.ID)

	// Check if referral is already used
	if referral.ReferredUserID != 0 {
		utils.LogError("Referral token already used - Token: %s, Used by User ID: %d", token, referral.ReferredUserID)
		utils.BadRequest(c, "Referral token already used", nil)
		return
	}
	utils.LogDebug("Referral token is available for use - Token: %s", token)

	// Generate coupon for referred user
	coupon := models.Coupon{
		Code:          "REF-NEW-" + referral.ReferralCode,
		Type:          "percent",
		Value:         10,
		MinOrderValue: 100,
		MaxDiscount:   200,
		Expiry:        time.Now().AddDate(0, 1, 0),
		UsageLimit:    1,
		Active:        true,
	}
	utils.LogDebug("Generated new coupon for referral - Code: %s", coupon.Code)

	// Start transaction
	tx := db.Begin()
	if err := tx.Create(&coupon).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to create coupon for referral - Token: %s: %v", token, err)
		utils.InternalServerError(c, "Failed to create coupon", err.Error())
		return
	}
	utils.LogDebug("Created coupon in transaction - Coupon ID: %d", coupon.ID)

	// Update referral with referred user
	referral.ReferredUserID = user.ID
	if err := tx.Save(&referral).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update referral with user - Token: %s, User ID: %d: %v", token, user.ID, err)
		utils.InternalServerError(c, "Failed to update referral", err.Error())
		return
	}
	utils.LogDebug("Updated referral with referred user - Token: %s, User ID: %d", token, user.ID)

	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit referral transaction - Token: %s: %v", token, err)
		utils.InternalServerError(c, "Failed to complete referral process", err.Error())
		return
	}
	utils.LogDebug("Successfully committed referral transaction - Token: %s", token)

	utils.LogInfo("Successfully accepted referral token - Token: %s, User ID: %d", token, user.ID)
	utils.Success(c, "Referral accepted successfully!", gin.H{
		"coupon_code":  coupon.Code,
		"discount":     coupon.Value,
		"min_order":    coupon.MinOrderValue,
		"max_discount": coupon.MaxDiscount,
		"expiry":       coupon.Expiry.Format("2006-01-02"),
	})
}

// AcceptReferralCodeAtSignup accepts a referral code during signup
func AcceptReferralCodeAtSignup(userID uint, code string) error {
	utils.LogInfo("AcceptReferralCodeAtSignup called - User ID: %d, Code: %s", userID, code)
	db := config.DB
	var referral models.Referral
	if err := db.Where("referral_code = ?", code).First(&referral).Error; err != nil {
		utils.LogError("Failed to find referral code - Code: %s: %v", code, err)
		return err
	}
	utils.LogDebug("Found referral record - Code: %s", code)

	// Create referral signup record
	refSignup := models.ReferralSignup{
		UserID:       userID,
		ReferralCode: code,
	}
	if err := db.Create(&refSignup).Error; err != nil {
		utils.LogError("Failed to create referral signup record - User ID: %d, Code: %s: %v", userID, code, err)
		return err
	}
	utils.LogInfo("Successfully created referral signup record - User ID: %d, Code: %s", userID, code)
	return nil
}
