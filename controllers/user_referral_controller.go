package controllers

import (
	"net/http"
	"time"
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// User: Accept referral via token URL (signup)
func AcceptReferralToken(c *gin.Context) {
	referToken := c.Param("token")
	var referral models.Referral
	db := config.DB
	if err := db.Where("referral_token = ?", referToken).First(&referral).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invalid referral token"})
		return
	}
	// Mark as used, generate coupon for referred user
	userVal, exists := c.Get("user")
	if !exists || userVal == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}
	user := userVal.(models.User)
	referral.ReferredUserID = user.ID
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
	db.Create(&coupon)
	referral.CouponID = coupon.ID
	db.Save(&referral)
	c.JSON(http.StatusOK, gin.H{"message": "Referral accepted! Coupon issued.", "coupon_code": coupon.Code})
}

// User: Accept referral via code during signup
func AcceptReferralCodeAtSignup(userID uint, code string) error {
	db := config.DB
	var referral models.Referral
	if err := db.Where("referral_code = ?", code).First(&referral).Error; err != nil {
		return err
	}
	refSignup := models.ReferralSignup{
		UserID: userID,
		ReferralCode: code,
	}
	db.Create(&refSignup)
	// Generate coupon for referred user
	coupon := models.Coupon{
		Code:          "REF-NEW-" + code,
		Type:          "percent",
		Value:         10,
		MinOrderValue: 100,
		MaxDiscount:   200,
		Expiry:        time.Now().AddDate(0, 1, 0),
		UsageLimit:    1,
		Active:        true,
	}
	db.Create(&coupon)
	return nil
}
