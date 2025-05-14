package controllers

import (
	"log"
	"os"
	"path/filepath"

	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// GetUserProfile returns the user's profile information
func GetUserProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.Success(c, "Profile retrieved successfully", gin.H{
		"user": gin.H{
			"username":      userModel.Username,
			"email":         userModel.Email,
			"first_name":    userModel.FirstName,
			"last_name":     userModel.LastName,
			"phone":         userModel.Phone,
			"profile_image": userModel.ProfileImage,
		},
	})
}

// UpdateProfileRequest represents the profile update request
type UpdateProfileRequest struct {
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

// UpdateProfile handles profile updates (excluding email)
func UpdateProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	updates := map[string]interface{}{}

	// Username validation and uniqueness
	if req.Username != "" && req.Username != userModel.Username {
		if valid, msg := utils.ValidateUsername(req.Username); !valid {
			utils.BadRequest(c, msg, nil)
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("username = ? AND id != ?", req.Username, userModel.ID).First(&existingUser).Error; err == nil {
			utils.Conflict(c, "Username already exists", nil)
			return
		}
		updates["username"] = req.Username
	}

	// First name
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			utils.BadRequest(c, msg, nil)
			return
		}
		updates["first_name"] = strings.TrimSpace(req.FirstName)
	}

	// Last name
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			utils.BadRequest(c, msg, nil)
			return
		}
		updates["last_name"] = strings.TrimSpace(req.LastName)
	}

	// Phone validation and uniqueness
	if req.Phone != "" && req.Phone != userModel.Phone {
		if valid, msg := utils.ValidatePhone(req.Phone); !valid {
			utils.BadRequest(c, msg, nil)
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("phone = ? AND id != ?", req.Phone, userModel.ID).First(&existingUser).Error; err == nil {
			utils.Conflict(c, "Phone number already exists", nil)
			return
		}
		updates["phone"] = req.Phone
	}

	if len(updates) == 0 {
		utils.BadRequest(c, "No valid fields to update", nil)
		return
	}

	// Update user
	if err := config.DB.Model(&userModel).Updates(updates).Error; err != nil {
		utils.InternalServerError(c, "Failed to update profile", err.Error())
		return
	}

	// Fetch updated user with wallet information
	var updatedUser models.User
	if err := config.DB.Preload("Wallet").First(&updatedUser, userModel.ID).Error; err != nil {
		utils.InternalServerError(c, "Failed to fetch updated profile", err.Error())
		return
	}

	utils.Success(c, "Profile updated successfully", gin.H{
		"user": gin.H{
			"id":            updatedUser.ID,
			"username":      updatedUser.Username,
			"email":         updatedUser.Email,
			"first_name":    updatedUser.FirstName,
			"last_name":     updatedUser.LastName,
			"phone":         updatedUser.Phone,
			"profile_image": updatedUser.ProfileImage,
			"is_verified":   updatedUser.IsVerified,
			"wallet": gin.H{
				"balance": updatedUser.Wallet.Balance,
			},
		},
	})
}

// UpdateEmailRequest represents the email update request
type UpdateEmailRequest struct {
	NewEmail string `json:"new_email" binding:"required,email"`
}

// UpdateEmail initiates the email update process
func UpdateEmail(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)

	var req UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.NewEmail); !valid {
		utils.BadRequest(c, msg, nil)
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := config.DB.Where("email = ?", req.NewEmail).First(&existingUser).Error; err == nil {
		utils.Conflict(c, "Email already exists", nil)
		return
	}

	// Generate OTP
	otp := utils.GenerateOTP()
	log.Printf("DEBUG: OTP for %s is %s", req.NewEmail, otp)
	otpExpiry := time.Now().Add(15 * time.Minute)

	// Store OTP
	if err := config.DB.Create(&models.UserOTP{
		UserID:    userModel.ID,
		Code:      otp,
		ExpiresAt: otpExpiry,
	}).Error; err != nil {
		utils.InternalServerError(c, "Failed to generate verification code", err.Error())
		return
	}

	// Send OTP email
	if err := utils.SendOTP(req.NewEmail, otp); err != nil {
		utils.InternalServerError(c, "Failed to send verification email", err.Error())
		return
	}

	// Store new email in session for verification
	session := sessions.Default(c)
	session.Set("new_email", req.NewEmail)
	session.Save()

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
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)

	var req VerifyEmailUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Get new email from session
	session := sessions.Default(c)
	newEmail := session.Get("new_email")
	if newEmail == nil {
		utils.BadRequest(c, "Email update not initiated", "Please initiate email update first")
		return
	}

	// Verify OTP
	var userOTP models.UserOTP
	if err := config.DB.Where("user_id = ? AND code = ? AND expires_at > ?",
		userModel.ID, req.OTP, time.Now()).First(&userOTP).Error; err != nil {
		utils.BadRequest(c, "Invalid or expired OTP", nil)
		return
	}

	// Update email
	if err := config.DB.Model(&userModel).Update("email", newEmail).Error; err != nil {
		utils.InternalServerError(c, "Failed to update email", err.Error())
		return
	}

	// Clear session
	session.Delete("new_email")
	session.Save()

	utils.Success(c, "Email updated successfully", gin.H{
		"user": gin.H{
			"id":    userModel.ID,
			"email": newEmail,
		},
	})
}

// ChangePasswordRequest represents the password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// ChangePassword handles password changes
func ChangePassword(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.CurrentPassword)); err != nil {
		utils.Unauthorized(c, "Current password is incorrect")
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		utils.BadRequest(c, msg, nil)
		return
	}

	// Check if new password matches confirm password
	if req.NewPassword != req.ConfirmPassword {
		utils.BadRequest(c, "New password and confirm password do not match", nil)
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.NewPassword)); err == nil {
		utils.BadRequest(c, "New password cannot be the same as current password", nil)
		return
	}

	// Check password history
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", userModel.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				utils.BadRequest(c, "This password has been used recently", "Please choose a different password that hasn't been used in your last 3 passwords")
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalServerError(c, "Failed to hash password", err.Error())
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Update password
	if err := tx.Model(&userModel).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update password", err.Error())
		return
	}

	// Add to password history
	passwordHistoryEntry := models.PasswordHistory{
		UserID:   userModel.ID,
		Password: string(hashedPassword),
	}
	if err := tx.Create(&passwordHistoryEntry).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update password history", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	utils.Success(c, "Password changed successfully", gin.H{
		"user": gin.H{
			"id": userModel.ID,
		},
		"redirect": gin.H{
			"url":     "/login",
			"message": "Please login again with your new password",
		},
	})
}

// UploadProfileImage handles profile image upload
func UploadProfileImage(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)

	// Get the file from the request
	file, err := c.FormFile("image")
	if err != nil {
		utils.BadRequest(c, "No file uploaded", "Please select an image file to upload")
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		utils.BadRequest(c, "Invalid file type", "Only jpg, jpeg, and png files are allowed")
		return
	}

	// Create uploads directory if it doesn't exist
	uploadDir := "uploads/profile_images"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.InternalServerError(c, "Failed to create upload directory", err.Error())
		return
	}

	// Generate unique filename
	filename := filepath.Join(uploadDir, userModel.Username+"_"+time.Now().Format("20060102150405")+ext)

	// Save the file
	if err := c.SaveUploadedFile(file, filename); err != nil {
		utils.InternalServerError(c, "Failed to save file", err.Error())
		return
	}

	// Update user's profile image
	if err := config.DB.Model(&userModel).Update("profile_image", filename).Error; err != nil {
		utils.InternalServerError(c, "Failed to update profile image", err.Error())
		return
	}

	utils.Success(c, "Profile image uploaded successfully", gin.H{
		"user": gin.H{
			"id":            userModel.ID,
			"profile_image": filename,
		},
	})
}
