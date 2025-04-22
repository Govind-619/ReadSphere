package controllers

import (
	"log"
	"net/http"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// ensureAddressesTableExists checks and creates the addresses table if it does not exist
func ensureAddressesTableExists() {
	db := config.DB
	type result struct{ TableName string }
	var res result
	db.Raw("SELECT to_regclass('public.addresses') AS table_name;").Scan(&res)
	if res.TableName == "" {
		log.Println("Table 'addresses' does not exist. Creating...")
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
	ensureAddressesTableExists()
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	var req AddAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Business validation
	errs := utils.ValidateAddressFields(req.Line1, req.Line2, req.City, req.State, req.Country, req.PostalCode, &req.IsDefault)
	if len(errs) > 0 {
		c.JSON(422, gin.H{"error": "Validation failed", "fields": errs})
		return
	}

	// Auto-formatting: capitalize city, state, country
	req.City = utils.Title(strings.ToLower(strings.TrimSpace(req.City)))
	req.State = utils.Title(strings.ToLower(strings.TrimSpace(req.State)))
	req.Country = utils.Title(strings.ToLower(strings.TrimSpace(req.Country)))

	// Unset previous default if needed
	if req.IsDefault {
		config.DB.Model(&models.Address{}).Where("user_id = ?", user.(models.User).ID).Update("is_default", false)
	}

	address := models.Address{
		UserID:     user.(models.User).ID,
		Line1:      req.Line1,
		Line2:      req.Line2,
		City:       req.City,
		State:      req.State,
		Country:    req.Country,
		PostalCode: req.PostalCode,
		IsDefault:  req.IsDefault,
	}

	if err := config.DB.Create(&address).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add address"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Address added successfully", "address": address})
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
	ensureAddressesTableExists()
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}

	addressID := c.Param("id")
	var req EditAddressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var address models.Address
	if err := config.DB.Where("id = ? AND user_id = ?", addressID, user.(models.User).ID).First(&address).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Address not found"})
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
		c.JSON(422, gin.H{"error": "Validation failed", "fields": errs})
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Address updated successfully", "address": address})
}

// DeleteAddress deletes an address for the user
func DeleteAddress(c *gin.Context) {
	ensureAddressesTableExists()
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	userModel := user.(models.User)
	addressID := c.Param("id")

	if err := config.DB.Where("id = ? AND user_id = ?", addressID, userModel.ID).Delete(&models.Address{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Address deleted successfully"})
}

// SetDefaultAddress sets one address as default for the user
func SetDefaultAddress(c *gin.Context) {
	ensureAddressesTableExists()
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	userModel := user.(models.User)
	addressID := c.Param("id")

	// Unset all previous defaults
	config.DB.Model(&models.Address{}).Where("user_id = ?", userModel.ID).Update("is_default", false)

	// Set this address as default
	if err := config.DB.Model(&models.Address{}).Where("id = ? AND user_id = ?", addressID, userModel.ID).Update("is_default", true).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default address"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default address set successfully"})
}

// GetAddresses returns all addresses for the authenticated user
func GetAddresses(c *gin.Context) {
	ensureAddressesTableExists()
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
		return
	}
	userModel := user.(models.User)

	var addresses []models.Address
	if err := config.DB.Where("user_id = ?", userModel.ID).Find(&addresses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch addresses"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"addresses": addresses})
}
