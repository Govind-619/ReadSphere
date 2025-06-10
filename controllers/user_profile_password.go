package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ChangePasswordRequest represents the password change request
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// ChangePassword handles password changes
func ChangePassword(c *gin.Context) {
	utils.LogInfo("ChangePassword called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Processing password change for user ID: %d", userModel.ID)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.CurrentPassword)); err != nil {
		utils.LogError("Current password verification failed for user ID: %d", userModel.ID)
		utils.Unauthorized(c, "Current password is incorrect")
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		utils.LogError("Password validation failed for user ID: %d: %s", userModel.ID, msg)
		utils.BadRequest(c, msg, nil)
		return
	}

	// Check if new password matches confirm password
	if req.NewPassword != req.ConfirmPassword {
		utils.LogError("Password confirmation mismatch for user ID: %d", userModel.ID)
		utils.BadRequest(c, "New password and confirm password do not match", nil)
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.NewPassword)); err == nil {
		utils.LogError("New password same as current password for user ID: %d", userModel.ID)
		utils.BadRequest(c, "New password cannot be the same as current password", nil)
		return
	}

	// Check password history
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", userModel.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				utils.LogError("Password recently used for user ID: %d", userModel.ID)
				utils.BadRequest(c, "This password has been used recently", "Please choose a different password that hasn't been used in your last 3 passwords")
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.LogError("Failed to hash password for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to hash password", err.Error())
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction for user ID: %d: %v", userModel.ID, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Update password
	if err := tx.Model(&userModel).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update password in database for user ID: %d: %v", userModel.ID, err)
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
		utils.LogError("Failed to update password history for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to update password history", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit transaction for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	utils.LogInfo("Password changed successfully for user ID: %d", userModel.ID)
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
