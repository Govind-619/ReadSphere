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
	token := c.Param("token")
	var referral models.Referral
	db := config.DB
	if err := db.Preload("Coupon").Where("referral_token = ?", token).First(&referral).Error; err != nil {
		utils.NotFound(c, "Invalid referral token")
		return
	}

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
	token := c.Param("token")
	var referral models.Referral
	db := config.DB
	if err := db.Where("referral_token = ?", token).First(&referral).Error; err != nil {
		utils.NotFound(c, "Invalid referral token")
		return
	}

	// Check if user is authenticated
	userVal, exists := c.Get("user")
	if !exists || userVal == nil {
		utils.Unauthorized(c, "User not authenticated")
		return
	}
	user := userVal.(models.User)

	// Check if referral is already used
	if referral.ReferredUserID != 0 {
		utils.BadRequest(c, "Referral token already used", nil)
		return
	}

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

	// Start transaction
	tx := db.Begin()
	if err := tx.Create(&coupon).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create coupon", err.Error())
		return
	}

	// Update referral with referred user
	referral.ReferredUserID = user.ID
	if err := tx.Save(&referral).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update referral", err.Error())
		return
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to complete referral process", err.Error())
		return
	}

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
	db := config.DB
	var referral models.Referral
	if err := db.Where("referral_code = ?", code).First(&referral).Error; err != nil {
		return err
	}

	// Create referral signup record
	refSignup := models.ReferralSignup{
		UserID:       userID,
		ReferralCode: code,
	}
	return db.Create(&refSignup).Error
}
