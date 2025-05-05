package controllers

import (
	"net/http"
	"time"
	"strings"
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
)

// Admin: Generate referral token URL and coupon for a user
func GenerateReferralToken(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	referToken := strings.ToUpper(RandomString(10))
	referralCode := strings.ToUpper(RandomString(6))

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
	db := config.DB
	db.Create(&coupon)

	referral := models.Referral{
		ReferrerUserID: req.UserID,
		ReferralCode:   referralCode,
		ReferralToken:  referToken,
		CouponID:       coupon.ID,
	}
	db.Create(&referral)

	c.JSON(http.StatusOK, gin.H{
		"referral_token_url": "/referral/invite/" + referToken,
		"referral_code": referralCode,
		"coupon_code": coupon.Code,
	})
}

// Admin: Get all referrals
func GetAllReferrals(c *gin.Context) {
	var referrals []models.Referral
	db := config.DB
	db.Preload("Coupon").Find(&referrals)
	c.JSON(http.StatusOK, referrals)
}

// Util: Generate random string
func RandomString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
