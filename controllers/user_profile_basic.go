package controllers

import (
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetUserProfile returns the user's profile information
func GetUserProfile(c *gin.Context) {
	utils.LogInfo("GetUserProfile called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("User profile retrieved for user ID: %d", userModel.ID)
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
	utils.LogInfo("UpdateProfile called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Updating profile for user ID: %d", userModel.ID)

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	updates := map[string]interface{}{}

	// Username validation and uniqueness
	if req.Username != "" && req.Username != userModel.Username {
		if valid, msg := utils.ValidateUsername(req.Username); !valid {
			utils.LogError("Invalid username: %s", msg)
			utils.BadRequest(c, msg, nil)
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("username = ? AND id != ?", req.Username, userModel.ID).First(&existingUser).Error; err == nil {
			utils.LogError("Username already exists: %s", req.Username)
			utils.Conflict(c, "Username already exists", nil)
			return
		}
		updates["username"] = req.Username
		utils.LogInfo("Username updated to: %s", req.Username)
	}

	// First name
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			utils.LogError("Invalid first name: %s", msg)
			utils.BadRequest(c, msg, nil)
			return
		}
		updates["first_name"] = strings.TrimSpace(req.FirstName)
		utils.LogInfo("First name updated to: %s", req.FirstName)
	}

	// Last name
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			utils.LogError("Invalid last name: %s", msg)
			utils.BadRequest(c, msg, nil)
			return
		}
		updates["last_name"] = strings.TrimSpace(req.LastName)
		utils.LogInfo("Last name updated to: %s", req.LastName)
	}

	// Phone validation and uniqueness
	if req.Phone != "" && req.Phone != userModel.Phone {
		if valid, msg := utils.ValidatePhone(req.Phone); !valid {
			utils.LogError("Invalid phone: %s", msg)
			utils.BadRequest(c, msg, nil)
			return
		}
		// Check uniqueness
		var existingUser models.User
		if err := config.DB.Where("phone = ? AND id != ?", req.Phone, userModel.ID).First(&existingUser).Error; err == nil {
			utils.LogError("Phone number already exists: %s", req.Phone)
			utils.Conflict(c, "Phone number already exists", nil)
			return
		}
		updates["phone"] = req.Phone
		utils.LogInfo("Phone updated to: %s", req.Phone)
	}

	if len(updates) == 0 {
		utils.LogError("No valid fields to update")
		utils.BadRequest(c, "No valid fields to update", nil)
		return
	}

	// Update user
	if err := config.DB.Model(&userModel).Updates(updates).Error; err != nil {
		utils.LogError("Failed to update profile: %v", err)
		utils.InternalServerError(c, "Failed to update profile", err.Error())
		return
	}

	// Fetch updated user with wallet information
	var updatedUser models.User
	if err := config.DB.Preload("Wallet").First(&updatedUser, userModel.ID).Error; err != nil {
		utils.LogError("Failed to fetch updated profile: %v", err)
		utils.InternalServerError(c, "Failed to fetch updated profile", err.Error())
		return
	}

	utils.LogInfo("Profile updated successfully for user ID: %d", updatedUser.ID)
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
