package controllers

import (
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// GetDeliveryCharges returns all delivery charges
func GetDeliveryCharges(c *gin.Context) {
	utils.LogInfo("GetDeliveryCharges called")

	var deliveryCharges []models.DeliveryCharge
	if err := config.DB.Find(&deliveryCharges).Error; err != nil {
		utils.LogError("Failed to fetch delivery charges: %v", err)
		utils.InternalServerError(c, "Failed to fetch delivery charges", err.Error())
		return
	}

	utils.Success(c, "Delivery charges retrieved successfully", gin.H{
		"delivery_charges": deliveryCharges,
	})
}

// AddDeliveryCharge adds a new delivery charge
func AddDeliveryCharge(c *gin.Context) {
	utils.LogInfo("AddDeliveryCharge called")

	var req struct {
		Pincode        string  `json:"pincode" binding:"required"`
		Charge         float64 `json:"charge" binding:"required,min=0"`
		MinOrderAmount float64 `json:"min_order_amount" binding:"min=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	deliveryCharge := models.DeliveryCharge{
		Pincode:        req.Pincode,
		Charge:         req.Charge,
		MinOrderAmount: req.MinOrderAmount,
		IsActive:       true,
	}

	if err := config.DB.Create(&deliveryCharge).Error; err != nil {
		utils.LogError("Failed to create delivery charge: %v", err)
		utils.InternalServerError(c, "Failed to create delivery charge", err.Error())
		return
	}

	utils.Success(c, "Delivery charge added successfully", gin.H{
		"delivery_charge": deliveryCharge,
	})
}

// UpdateDeliveryCharge updates an existing delivery charge
func UpdateDeliveryCharge(c *gin.Context) {
	utils.LogInfo("UpdateDeliveryCharge called")

	id := c.Param("id")
	deliveryChargeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.LogError("Invalid delivery charge ID: %s", id)
		utils.BadRequest(c, "Invalid delivery charge ID", nil)
		return
	}

	var req struct {
		Charge         *float64 `json:"charge"`
		MinOrderAmount *float64 `json:"min_order_amount"`
		IsActive       *bool    `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", err.Error())
		return
	}

	var deliveryCharge models.DeliveryCharge
	if err := config.DB.First(&deliveryCharge, deliveryChargeID).Error; err != nil {
		utils.LogError("Delivery charge not found: %v", err)
		utils.NotFound(c, "Delivery charge not found")
		return
	}

	updates := make(map[string]interface{})
	if req.Charge != nil {
		updates["charge"] = *req.Charge
	}
	if req.MinOrderAmount != nil {
		updates["min_order_amount"] = *req.MinOrderAmount
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	if err := config.DB.Model(&deliveryCharge).Updates(updates).Error; err != nil {
		utils.LogError("Failed to update delivery charge: %v", err)
		utils.InternalServerError(c, "Failed to update delivery charge", err.Error())
		return
	}

	utils.Success(c, "Delivery charge updated successfully", gin.H{
		"delivery_charge": deliveryCharge,
	})
}

// DeleteDeliveryCharge deletes a delivery charge
func DeleteDeliveryCharge(c *gin.Context) {
	utils.LogInfo("DeleteDeliveryCharge called")

	id := c.Param("id")
	deliveryChargeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.LogError("Invalid delivery charge ID: %s", id)
		utils.BadRequest(c, "Invalid delivery charge ID", nil)
		return
	}

	var deliveryCharge models.DeliveryCharge
	if err := config.DB.First(&deliveryCharge, deliveryChargeID).Error; err != nil {
		utils.LogError("Delivery charge not found: %v", err)
		utils.NotFound(c, "Delivery charge not found")
		return
	}

	if err := config.DB.Delete(&deliveryCharge).Error; err != nil {
		utils.LogError("Failed to delete delivery charge: %v", err)
		utils.InternalServerError(c, "Failed to delete delivery charge", err.Error())
		return
	}

	utils.Success(c, "Delivery charge deleted successfully", nil)
}

// GetDeliveryChargeByPincode returns delivery charge for a specific pincode
func GetDeliveryChargeByPincode(c *gin.Context) {
	utils.LogInfo("GetDeliveryChargeByPincode called")

	pincode := c.Param("pincode")
	if pincode == "" {
		utils.LogError("Pincode parameter is required")
		utils.BadRequest(c, "Pincode parameter is required", nil)
		return
	}

	var deliveryCharge models.DeliveryCharge
	if err := config.DB.Where("pincode = ? AND is_active = ?", pincode, true).First(&deliveryCharge).Error; err != nil {
		utils.LogError("Delivery charge not found for pincode %s: %v", pincode, err)
		utils.NotFound(c, "Delivery charge not found for this pincode")
		return
	}

	utils.Success(c, "Delivery charge retrieved successfully", gin.H{
		"delivery_charge": deliveryCharge,
	})
}
