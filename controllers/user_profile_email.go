package controllers

import (
	"log"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// UpdateEmailRequest represents the email update request
type UpdateEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

// UpdateEmail initiates the email update process
func UpdateEmail(c *gin.Context) {
	utils.LogInfo("UpdateEmail called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Initiating email update for user ID: %d", userModel.ID)

	var req UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.NewEmail); !valid {
		utils.LogError("Invalid email format: %s", msg)
		utils.BadRequest(c, msg, nil)
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := config.DB.Where("email = ?", req.NewEmail).First(&existingUser).Error; err == nil {
		utils.LogError("Email already exists: %s", req.NewEmail)
		utils.Conflict(c, "Email already exists", nil)
		return
	}

	// Generate OTP
	otp := utils.GenerateOTP()
	log.Printf("DEBUG: OTP for %s is %s", req.NewEmail, otp)
	utils.LogInfo("Generated OTP for email update: %s", req.NewEmail)
	otpExpiry := time.Now().Add(15 * time.Minute)

	// Store OTP
	if err := config.DB.Create(&models.UserOTP{
		UserID:    userModel.ID,
		Code:      otp,
		ExpiresAt: otpExpiry,
	}).Error; err != nil {
		utils.LogError("Failed to store OTP: %v", err)
		utils.InternalServerError(c, "Failed to generate verification code", err.Error())
		return
	}

	// Send OTP email
	if err := utils.SendOTP(req.NewEmail, otp); err != nil {
		utils.LogError("Failed to send OTP email: %v", err)
		utils.InternalServerError(c, "Failed to send verification email", err.Error())
		return
	}

	// Store new email in session for verification
	session := sessions.Default(c)
	session.Set("new_email", req.NewEmail)
	session.Save()

	utils.LogInfo("Email update initiated successfully for user ID: %d, new email: %s", userModel.ID, req.NewEmail)
	utils.Success(c, "Verification code sent to new email address", gin.H{
		"email":      req.NewEmail,
		"expires_in": 900, // 15 minutes in seconds
	})
}

// VerifyEmailUpdateRequest represents the email update verification request
type VerifyEmailUpdateRequest struct {
	OTP string `json:"otp" binding:"required"`
}

// VerifyEmailUpdate verifies the OTP and updates the email
func VerifyEmailUpdate(c *gin.Context) {
	utils.LogInfo("VerifyEmailUpdate called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Verifying email update for user ID: %d", userModel.ID)

	var req VerifyEmailUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Get new email from session
	session := sessions.Default(c)
	newEmail := session.Get("new_email")
	if newEmail == nil {
		utils.LogError("Email update not initiated for user ID: %d", userModel.ID)
		utils.BadRequest(c, "Email update not initiated", "Please initiate email update first")
		return
	}

	// Verify OTP
	var userOTP models.UserOTP
	if err := config.DB.Where("user_id = ? AND code = ? AND expires_at > ?",
		userModel.ID, req.OTP, time.Now()).First(&userOTP).Error; err != nil {
		utils.LogError("Invalid or expired OTP for user ID: %d", userModel.ID)
		utils.BadRequest(c, "Invalid or expired OTP", nil)
		return
	}

	// Update email
	if err := config.DB.Model(&userModel).Update("email", newEmail).Error; err != nil {
		utils.LogError("Failed to update email in database: %v", err)
		utils.InternalServerError(c, "Failed to update email", err.Error())
		return
	}

	// Clear session
	session.Delete("new_email")
	session.Save()

	utils.LogInfo("Email updated successfully for user ID: %d, new email: %s", userModel.ID, newEmail)
	utils.Success(c, "Email updated successfully", gin.H{
		"user": gin.H{
			"id":    userModel.ID,
			"email": newEmail,
		},
	})
}
