package controllers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)
	c.JSON(http.StatusOK, gin.H{
		"username":      userModel.Username,
		"email":         userModel.Email,
		"first_name":    userModel.FirstName,
		"last_name":     userModel.LastName,
		"phone":         userModel.Phone,
		"profile_image": userModel.ProfileImage,
		"address":       userModel.Address,
		"city":          userModel.City,
		"state":         userModel.State,
		"country":       userModel.Country,
		"postal_code":   userModel.PostalCode,
	})
}

// UpdateProfileRequest represents the profile update request
type UpdateProfileRequest struct {
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
}

// UpdateProfile handles profile updates (excluding email)
func UpdateProfile(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	updates := map[string]interface{}{}

	// Username validation and uniqueness
	if req.Username != "" && req.Username != userModel.Username {
		if valid, msg := utils.ValidateUsername(req.Username); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("username = ? AND id != ?", req.Username, userModel.ID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
		updates["username"] = req.Username
	}

	// First name
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		updates["first_name"] = strings.TrimSpace(req.FirstName)
	}

	// Last name
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		updates["last_name"] = strings.TrimSpace(req.LastName)
	}

	// Phone validation and uniqueness
	if req.Phone != "" && req.Phone != userModel.Phone {
		if valid, msg := utils.ValidatePhone(req.Phone); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("phone = ? AND id != ?", req.Phone, userModel.ID).First(&existingUser).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{"error": "Phone number already exists"})
			return
		}
		updates["phone"] = req.Phone
	}

	// Address, city, state, country: trim and validate length
	if req.Address != "" {
		addr := strings.TrimSpace(req.Address)
		if err := utils.ValidateStringLength(addr, 2, 100); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address " + err.Error()})
			return
		}
		updates["address"] = addr
	}
	if req.City != "" {
		city := strings.TrimSpace(req.City)
		if err := utils.ValidateStringLength(city, 2, 100); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "City " + err.Error()})
			return
		}
		updates["city"] = city
	}
	if req.State != "" {
		state := strings.TrimSpace(req.State)
		if err := utils.ValidateStringLength(state, 2, 100); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "State " + err.Error()})
			return
		}
		updates["state"] = state
	}
	if req.Country != "" {
		country := strings.TrimSpace(req.Country)
		if err := utils.ValidateStringLength(country, 2, 100); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Country " + err.Error()})
			return
		}
		updates["country"] = country
	}

	// Postal code: numeric and length (6 for India)
	if req.PostalCode != "" {
		postal := strings.TrimSpace(req.PostalCode)
		if _, err := strconv.Atoi(postal); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Postal code must be numeric"})
			return
		}
		if len(postal) != 6 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Postal code must be 6 digits"})
			return
		}
		updates["postal_code"] = postal
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid fields to update"})
		return
	}

	// Audit log placeholder
	// utils.LogAudit(userModel.ID, "profile_update", updates)

	if err := config.DB.Model(&userModel).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Profile updated successfully",
		"user":    userModel,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)

	var req UpdateEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.NewEmail); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if email already exists
	var existingUser models.User
	if err := config.DB.Where("email = ?", req.NewEmail).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Email already exists"})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate verification code"})
		return
	}

	// Send OTP email
	if err := utils.SendOTP(req.NewEmail, otp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	// Store new email in session for verification
	session := sessions.Default(c)
	session.Set("new_email", req.NewEmail)
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"message": "Verification code sent to new email address",
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)

	var req VerifyEmailUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Get new email from session
	session := sessions.Default(c)
	newEmail := session.Get("new_email")
	if newEmail == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Email update not initiated"})
		return
	}

	// Verify OTP
	var userOTP models.UserOTP
	if err := config.DB.Where("user_id = ? AND code = ? AND expires_at > ?",
		userModel.ID, req.OTP, time.Now()).First(&userOTP).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired OTP"})
		return
	}

	// Update email
	if err := config.DB.Model(&userModel).Update("email", newEmail).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update email"})
		return
	}

	// Clear session
	session.Delete("new_email")
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"message": "Email updated successfully",
		"user":    userModel,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)

	var req ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.CurrentPassword)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Current password is incorrect"})
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if new password matches confirm password
	if req.NewPassword != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password and confirm password do not match"})
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(userModel.Password), []byte(req.NewPassword)); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password cannot be the same as current password"})
		return
	}

	// Check password history
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", userModel.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "This password has been used recently"})
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	// Update password
	if err := tx.Model(&userModel).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Add to password history
	passwordHistoryEntry := models.PasswordHistory{
		UserID:   userModel.ID,
		Password: string(hashedPassword),
	}
	if err := tx.Create(&passwordHistoryEntry).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password history"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password changed successfully",
	})
}

// UploadProfileImage handles profile image upload
func UploadProfileImage(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	userModel := user.(models.User)

	// Get the file from the request
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only jpg, jpeg, and png files are allowed"})
		return
	}

	// Create uploads directory if it doesn't exist
	uploadDir := "uploads/profile_images"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create upload directory"})
		return
	}

	// Generate unique filename
	filename := filepath.Join(uploadDir, userModel.Username+"_"+time.Now().Format("20060102150405")+ext)

	// Save the file
	if err := c.SaveUploadedFile(file, filename); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	// Update user's profile image
	if err := config.DB.Model(&userModel).Update("profile_image", filename).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update profile image"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Profile image uploaded successfully",
		"image_url": filename,
	})
}
