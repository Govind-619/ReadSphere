package controllers

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func UserLogout(c *gin.Context) {
	// Get user ID before clearing session
	userID, exists := c.Get("user_id")
	if exists {
		utils.LogInfo("User %d logging out", userID)
	}

	// Clear session
	session := sessions.Default(c)
	session.Clear()
	err := session.Save()
	if err != nil {
		utils.LogError("Failed to save cleared session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to logout"})
		return
	}

	// Get the token from the request
	token := c.GetHeader("Authorization")
	if token != "" {
		// Remove "Bearer " prefix if present
		token = strings.TrimPrefix(token, "Bearer ")

		// Add token to blacklist
		blacklistedToken := models.BlacklistedToken{
			Token:     token,
			ExpiresAt: time.Now().Add(24 * time.Hour), // Same as JWT expiration
		}

		if err := config.DB.Create(&blacklistedToken).Error; err != nil {
			utils.LogError("Failed to blacklist token: %v", err)
		} else {
			utils.LogInfo("Token blacklisted for user %d", userID)
		}
	}

	utils.LogInfo("User session cleared and token blacklisted successfully")
	utils.Success(c, "Logout successful", nil)
}

func AddReview(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("user_id")
	if !exists {
		utils.LogError("Unauthorized review attempt - no user ID found")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Log the review attempt
	utils.LogInfo("User %d attempting to add a review", userID)

	// TODO: Add actual review logic here
	// For now, just log the success
	utils.LogInfo("Review added successfully by user %d", userID)
	c.JSON(http.StatusOK, gin.H{"message": "Review added successfully"})
}

func generateOTP() string {
	utils.LogDebug("Generating new OTP")
	// Use crypto/rand for secure random number generation
	b := make([]byte, 6)
	for i := 0; i < 6; i++ {
		num := 0
		for {
			r, err := rand.Int(rand.Reader, big.NewInt(10))
			if err == nil {
				num = int(r.Int64())
				break
			}
		}
		b[i] = byte('0' + num)
	}
	utils.LogDebug("OTP generated successfully")
	return string(b)
}
