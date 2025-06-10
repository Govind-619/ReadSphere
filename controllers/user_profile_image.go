package controllers

import (
	"os"
	"path/filepath"

	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// UploadProfileImage handles profile image upload
func UploadProfileImage(c *gin.Context) {
	utils.LogInfo("UploadProfileImage called")

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Processing profile image upload for user ID: %d", userModel.ID)

	// Get the file from the request
	file, err := c.FormFile("image")
	if err != nil {
		utils.LogError("No file uploaded for user ID: %d", userModel.ID)
		utils.BadRequest(c, "No file uploaded", "Please select an image file to upload")
		return
	}

	// Validate file type
	ext := filepath.Ext(file.Filename)
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		utils.LogError("Invalid file type for user ID: %d: %s", userModel.ID, ext)
		utils.BadRequest(c, "Invalid file type", "Only jpg, jpeg, and png files are allowed")
		return
	}

	// Create uploads directory if it doesn't exist
	uploadDir := "uploads/profile_images"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		utils.LogError("Failed to create upload directory for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to create upload directory", err.Error())
		return
	}

	// Generate unique filename
	filename := filepath.Join(uploadDir, userModel.Username+"_"+time.Now().Format("20060102150405")+ext)
	utils.LogInfo("Generated filename for user ID: %d: %s", userModel.ID, filename)

	// Save the file
	if err := c.SaveUploadedFile(file, filename); err != nil {
		utils.LogError("Failed to save file for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to save file", err.Error())
		return
	}

	// Update user's profile image
	if err := config.DB.Model(&userModel).Update("profile_image", filename).Error; err != nil {
		utils.LogError("Failed to update profile image in database for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to update profile image", err.Error())
		return
	}

	utils.LogInfo("Profile image uploaded successfully for user ID: %d", userModel.ID)
	utils.Success(c, "Profile image uploaded successfully", gin.H{
		"user": gin.H{
			"id":            userModel.ID,
			"profile_image": filename,
		},
	})
}
