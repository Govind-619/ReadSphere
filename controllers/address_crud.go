package controllers

import (
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ensureAddressesTableExists checks and creates the addresses table if it does not exist
func ensureAddressesTableExists() {
	utils.LogInfo("Checking if addresses table exists")
	db := config.DB
	type result struct{ TableName string }
	var res result
	db.Raw("SELECT to_regclass('public.addresses') AS table_name;").Scan(&res)
	if res.TableName == "" {
		utils.LogInfo("Table 'addresses' does not exist. Creating...")
		db.Exec(`CREATE TABLE IF NOT EXISTS addresses (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL,
			line1 VARCHAR(255),
			line2 VARCHAR(255),
			city VARCHAR(100),
			state VARCHAR(100),
			country VARCHAR(100),
			postal_code VARCHAR(20),
			is_default BOOLEAN DEFAULT FALSE,
			created_at TIMESTAMP,
			updated_at TIMESTAMP
		);`)
		utils.LogInfo("Addresses table created successfully")
	}
}

// AddAddress adds a new address for the user
type AddAddressRequest struct {
	Line1      string `json:"line1" binding:"required"`
	Line2      string `json:"line2"`
	City       string `json:"city" binding:"required"`
	State      string `json:"state" binding:"required"`
	Country    string `json:"country" binding:"required"`
	PostalCode string `json:"postal_code" binding:"required"`
	IsDefault  bool   `json:"is_default"`
}

func AddAddress(c *gin.Context) {
	utils.LogInfo("AddAddress called")
	ensureAddressesTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	utils.LogInfo("Processing address addition for user ID: %d", userModel.ID)

	var req AddAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userModel.ID, err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	// Business validation
	errs := utils.ValidateAddressFields(req.Line1, req.Line2, req.City, req.State, req.Country, req.PostalCode, &req.IsDefault)
	if len(errs) > 0 {
		utils.LogError("Address validation failed for user ID: %d: %v", userModel.ID, errs)
		utils.BadRequest(c, "Validation failed", gin.H{"fields": errs})
		return
	}

	// Auto-formatting: capitalize city, state, country
	req.City = utils.Title(strings.ToLower(strings.TrimSpace(req.City)))
	req.State = utils.Title(strings.ToLower(strings.TrimSpace(req.State)))
	req.Country = utils.Title(strings.ToLower(strings.TrimSpace(req.Country)))

	// Unset previous default if needed
	if req.IsDefault {
		if err := config.DB.Model(&models.Address{}).Where("user_id = ?", userModel.ID).Update("is_default", false).Error; err != nil {
			utils.LogError("Failed to update previous default address for user ID: %d: %v", userModel.ID, err)
			utils.InternalServerError(c, "Failed to update previous default address", err.Error())
			return
		}
	}

	address := models.Address{
		UserID:     userModel.ID,
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		Country:    req.Country,
		PostalCode: req.PostalCode,
		IsDefault:  req.IsDefault,
	}

	if err := config.DB.Create(&address).Error; err != nil {
		utils.LogError("Failed to add address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to add address", err.Error())
		return
	}

	// Query the created address without timestamp fields
	var createdAddress struct {
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
		First(&createdAddress).Error; err != nil {
		utils.LogError("Failed to fetch created address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to fetch created address", err.Error())
		return
	}

	utils.LogInfo("Address added successfully for user ID: %d, address ID: %d", userModel.ID, address.ID)
	utils.Success(c, "Address added successfully", gin.H{
		"address": createdAddress,
	})
}

// EditAddress edits an existing address for the user
type EditAddressRequest struct {
	Line1      string `json:"line1"`
	Line2      string `json:"line2"`
	City       string `json:"city"`
	State      string `json:"state"`
	Country    string `json:"country"`
	PostalCode string `json:"postal_code"`
}

func EditAddress(c *gin.Context) {
	utils.LogInfo("EditAddress called")
	ensureAddressesTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}

	userModel := user.(models.User)
	addressID := c.Param("id")
	utils.LogInfo("Processing address edit for user ID: %d, address ID: %s", userModel.ID, addressID)

	var req EditAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userModel.ID, err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", addressID, userModel.ID).First(&address).Error; err != nil {
		utils.LogError("Address not found for user ID: %d, address ID: %s", userModel.ID, addressID)
		utils.NotFound(c, "Address not found")
		return
	}

	// Business validation (use existing values if fields not provided)
	line1 := req.Line1
	if line1 == "" {
		line1 = address.Line1
	}
	line2 := req.Line2
	if line2 == "" {
		line2 = address.Line2
	}
	city := req.City
	if city == "" {
		city = address.City
	}
	state := req.State
	if state == "" {
		state = address.State
	}
	country := req.Country
	if country == "" {
		country = address.Country
	}
	postalCode := req.PostalCode
	if postalCode == "" {
		postalCode = address.PostalCode
	}
	errs := utils.ValidateAddressFields(line1, line2, city, state, country, postalCode, nil)
	if len(errs) > 0 {
		utils.LogError("Address validation failed for user ID: %d: %v", userModel.ID, errs)
		utils.BadRequest(c, "Validation failed", gin.H{"fields": errs})
		return
	}

	// Auto-formatting: capitalize city, state, country
	if req.City != "" {
		req.City = utils.Title(strings.ToLower(strings.TrimSpace(req.City)))
	}
	if req.State != "" {
		req.State = utils.Title(strings.ToLower(strings.TrimSpace(req.State)))
	}
	if req.Country != "" {
		req.Country = utils.Title(strings.ToLower(strings.TrimSpace(req.Country)))
	}

	// Update fields if provided
	if req.Line1 != "" {
		address.Line1 = req.Line1
	}
	if req.Line2 != "" {
		address.Line2 = req.Line2
	}
	if req.City != "" {
		address.City = req.City
	}
	if req.State != "" {
		address.State = req.State
	}
	if req.Country != "" {
		address.Country = req.Country
	}
	if req.PostalCode != "" {
		address.PostalCode = req.PostalCode
	}

	if err := config.DB.Save(&address).Error; err != nil {
		utils.LogError("Failed to update address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to update address", err.Error())
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

	utils.LogInfo("Address updated successfully for user ID: %d, address ID: %d", userModel.ID, address.ID)
	utils.Success(c, "Address updated successfully", gin.H{
		"address": updatedAddress,
	})
}

// DeleteAddress deletes an address for the user
func DeleteAddress(c *gin.Context) {
	utils.LogInfo("DeleteAddress called")
	ensureAddressesTableExists()

	user, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found in context")
		return
	}
	userModel := user.(models.User)
	addressID := c.Param("id")
	utils.LogInfo("Processing address deletion for user ID: %d, address ID: %s", userModel.ID, addressID)

	// First check if address exists and belongs to user
	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", addressID, userModel.ID).First(&address).Error; err != nil {
		utils.LogError("Address not found or already deleted for user ID: %d, address ID: %s", userModel.ID, addressID)
		utils.NotFound(c, "Address not found or already deleted")
		return
	}

	// Check if address is referenced in any orders
	var orderCount int64
	if err := config.DB.Model(&models.Order{}).Where("address_id = ?", addressID).Count(&orderCount).Error; err != nil {
		utils.LogError("Failed to check order references for address ID: %s: %v", addressID, err)
		utils.InternalServerError(c, "Failed to check address usage", err.Error())
		return
	}

	if orderCount > 0 {
		utils.LogError("Cannot delete address ID: %s as it is referenced by %d orders", addressID, orderCount)
		utils.BadRequest(c, "Cannot delete address", "This address is associated with existing orders and cannot be deleted")
		return
	}

	// Perform the delete operation
	if err := config.DB.Delete(&address).Error; err != nil {
		utils.LogError("Failed to delete address for user ID: %d: %v", userModel.ID, err)
		utils.InternalServerError(c, "Failed to delete address", err.Error())
		return
	}

	utils.LogInfo("Address deleted successfully for user ID: %d, address ID: %d", userModel.ID, address.ID)
	utils.Success(c, "Address deleted successfully", nil)
}
