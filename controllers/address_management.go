package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// SetDefaultAddress sets one address as default for the user
func SetDefaultAddress(c *gin.Context) {
	utils.LogInfo("SetDefaultAddress called")
	ensureAddressesTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}
	userModel := user.(models.User)
	addressID := c.Param("id")
	utils.LogInfo("Processing default address setting for user ID: %d, address ID: %s", userModel.ID, addressID)

	// First check if address exists and belongs to user
	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", addressID, userModel.ID).First(&address).Error; err != nil {
		utils.LogError("Address not found for user ID: %d, address ID: %s", userModel.ID, addressID)
		utils.NotFound(c, "Address not found")
		return
	}

	// Start a transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Failed to start transaction for user ID: %d: %v", userModel.ID, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", tx.Error.Error())
		return
	}

	// Unset all previous defaults
	if err := tx.Model(&models.Address{}).Where("user_id = ?", userModel.ID).Update("is_default", false).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to update previous default addresses for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to update previous default addresses", err.Error())
		return
	}

	// Set this address as default
	if err := tx.Model(&address).Update("is_default", true).Error; err != nil {
		tx.Rollback()
		utils.LogError("Failed to set default address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to set default address", err.Error())
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Failed to commit changes for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to commit changes", err.Error())
		return
	}

	// Query the updated address without timestamp fields
	var updatedAddress struct {
		ID         uint   `json:"id"`
		UserID     uint   `json:"user_id"`
		Line1      string `json:"line1"`
		Line2      string `json:"line2"`
		City       string `json:"city"`
		State      string `json:"state"`
		Country    string `json:"country"`
		PostalCode string `json:"postal_code"`
		IsDefault  bool   `json:"is_default"`
	}

	if err := config.DB.Table("addresses").
		Select("id, user_id, line1, line2, city, state, country, postal_code, is_default").
		Where("id = ?", address.ID).
		First(&updatedAddress).Error; err != nil {
		utils.LogError("Failed to fetch updated address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to fetch updated address", err.Error())
		return
	}

	utils.LogInfo("Default address set successfully for user ID: %d, address ID: %d", userModel.ID, address.ID)
	utils.Success(c, "Default address set successfully", gin.H{
		"address": updatedAddress,
	})
}

// GetAddresses returns all addresses for the authenticated user
func GetAddresses(c *gin.Context) {
	utils.LogInfo("GetAddresses called")
	ensureAddressesTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}
	userModel := user.(models.User)
	utils.LogInfo("Fetching addresses for user ID: %d", userModel.ID)

	var addresses []struct {
		ID         uint   `json:"id"`
		UserID     uint   `json:"user_id"`
		Line1      string `json:"line1"`
		Line2      string `json:"line2"`
		City       string `json:"city"`
		State      string `json:"state"`
		Country    string `json:"country"`
		PostalCode string `json:"postal_code"`
		IsDefault  bool   `json:"is_default"`
	}

	if err := config.DB.Table("addresses").
		Select("id, user_id, line1, line2, city, state, country, postal_code, is_default").
		Where("user_id = ?", userModel.ID).
		Find(&addresses).Error; err != nil {
		utils.LogError("Failed to fetch addresses for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to fetch addresses", err.Error())
		return
	}

	utils.LogInfo("Successfully retrieved %d addresses for user ID: %d", len(addresses), userModel.ID)
	utils.Success(c, "Addresses retrieved successfully", gin.H{
		"addresses": addresses,
	})
}
