package utils

import (
	"fmt"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// GetDeliveryCharge calculates delivery charge based on pincode and order amount
func GetDeliveryCharge(pincode string, orderAmount float64) (float64, error) {
	db := config.DB

	// Debug: Log the pincode being searched
	LogInfo("Searching for delivery charge for pincode: %s, order amount: %.2f", pincode, orderAmount)

	// Find delivery charge for the specific pincode
	var deliveryCharge models.DeliveryCharge
	if err := db.Where("pincode = ? AND is_active = ?", pincode, true).
		First(&deliveryCharge).Error; err == nil {

		LogInfo("Found delivery charge: %.2f for pincode %s", deliveryCharge.Charge, pincode)
		return deliveryCharge.Charge, nil
	}

	// Debug: Log when pincode not found
	LogInfo("Pincode %s not found in delivery_charges table, using default charge: 50.0", pincode)

	// Default delivery charge if pincode not found
	return 50.0, nil
}

// IsDeliveryAvailable checks if delivery is available to the given pincode
func IsDeliveryAvailable(pincode string) bool {
	db := config.DB

	// Check if there's a delivery charge rule for this pincode
	var count int64
	db.Model(&models.DeliveryCharge{}).
		Where("pincode = ? AND is_active = ?", pincode, true).
		Count(&count)

	return count > 0
}

// GetDeliveryChargeBreakdown returns detailed delivery charge information
func GetDeliveryChargeBreakdown(pincode string, orderAmount float64) (map[string]interface{}, error) {
	charge, err := GetDeliveryCharge(pincode, orderAmount)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"delivery_charge":     charge,
		"delivery_available":  true,
		"pincode":             pincode,
		"order_amount":        orderAmount,
		"total_with_delivery": orderAmount + charge,
	}, nil
}

// GetDeliveryChargeByPincode returns delivery charge info for a specific pincode
func GetDeliveryChargeByPincode(pincode string) (*models.DeliveryCharge, error) {
	db := config.DB

	var deliveryCharge models.DeliveryCharge
	if err := db.Where("pincode = ? AND is_active = ?", pincode, true).
		First(&deliveryCharge).Error; err != nil {
		return nil, err
	}

	return &deliveryCharge, nil
}

// DebugDeliveryChargesTable logs the status of the delivery_charges table
func DebugDeliveryChargesTable() {
	db := config.DB

	var count int64
	if err := db.Model(&models.DeliveryCharge{}).Count(&count).Error; err != nil {
		LogError("Failed to count delivery charges: %v", err)
		return
	}

	LogInfo("Total delivery charges in database: %d", count)

	if count > 0 {
		var sampleCharges []models.DeliveryCharge
		if err := db.Limit(5).Find(&sampleCharges).Error; err != nil {
			LogError("Failed to fetch sample delivery charges: %v", err)
			return
		}

		LogInfo("Sample delivery charges:")
		for _, charge := range sampleCharges {
			LogInfo("  Pincode: %s, Charge: %.2f, Min Order: %.2f, Active: %t",
				charge.Pincode, charge.Charge, charge.MinOrderAmount, charge.IsActive)
		}
	}
}

// GetFreeDeliveryInfo returns information about delivery charge
func GetFreeDeliveryInfo(pincode string, orderAmount float64) map[string]interface{} {
	// Check specific pincode charges
	charge, err := GetDeliveryChargeByPincode(pincode)
	if err != nil {
		return map[string]interface{}{
			"eligible":       false,
			"message":        "Delivery not available for this pincode",
			"regular_charge": 50.0,
			"current_charge": 50.0,
		}
	}

	return map[string]interface{}{
		"eligible":       true,
		"regular_charge": charge.Charge,
		"current_charge": charge.Charge,
		"message":        fmt.Sprintf("Delivery charge: ₹%.2f", charge.Charge),
	}
}
