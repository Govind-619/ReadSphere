package controllers

import (
	"fmt"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CheckoutSummary struct {
	Items      []gin.H `json:"items"`
	Subtotal   float64 `json:"subtotal"`
	Discount   float64 `json:"discount_total"`
	FinalTotal float64 `json:"final_total"`
}

func GetCheckoutSummary(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}

	// Get cart details using the same utility function
	cartDetails, err := utils.GetCartDetails(user.ID)
	if err != nil {
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}

	// Format items for response
	var items []gin.H
	for _, item := range cartDetails.OrderItems {
		offerBreakdown, _ := utils.GetOfferBreakdownForBook(item.BookID, item.Book.CategoryID)
		items = append(items, gin.H{
			"book_id":                item.BookID,
			"name":                   item.Book.Name,
			"image_url":              item.Book.ImageURL,
			"quantity":               item.Quantity,
			"original_price":         fmt.Sprintf("%.2f", item.Price),
			"product_offer_percent":  offerBreakdown.ProductOfferPercent,
			"category_offer_percent": offerBreakdown.CategoryOfferPercent,
			"applied_offer_percent":  offerBreakdown.ProductOfferPercent + offerBreakdown.CategoryOfferPercent,
			"applied_offer_type":     "product+category",
			"final_unit_price":       fmt.Sprintf("%.2f", item.Total/float64(item.Quantity)),
			"item_total":             fmt.Sprintf("%.2f", item.Total),
			"discount_amount":        fmt.Sprintf("%.2f", item.Discount),
		})
	}

	// Get wallet balance
	wallet, err := getOrCreateWallet(user.ID)
	var walletBalance float64 = 0
	if err == nil {
		walletBalance = wallet.Balance
	}

	utils.Success(c, "Checkout summary retrieved successfully", gin.H{
		"can_checkout":      len(items) > 0,
		"cart":              items,
		"subtotal":          fmt.Sprintf("%.2f", cartDetails.Subtotal),
		"product_discount":  fmt.Sprintf("%.2f", cartDetails.ProductDiscount),
		"category_discount": fmt.Sprintf("%.2f", cartDetails.CategoryDiscount),
		"coupon_code":       cartDetails.CouponCode,
		"coupon_discount":   fmt.Sprintf("%.2f", cartDetails.CouponDiscount),
		"final_total":       fmt.Sprintf("%.2f", cartDetails.FinalTotal),
		"total_discount":    fmt.Sprintf("%.2f", cartDetails.ProductDiscount+cartDetails.CategoryDiscount+cartDetails.CouponDiscount),
		"wallet_balance":    fmt.Sprintf("%.2f", walletBalance),
		"can_use_wallet":    walletBalance >= cartDetails.FinalTotal,
	})
}

func PlaceOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID

	var req struct {
		AddressID     uint            `json:"address_id"`
		Address       *models.Address `json:"address"`
		PaymentMethod string          `json:"payment_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Validate and normalize payment method
	paymentMethod := strings.ToLower(strings.TrimSpace(req.PaymentMethod))
	validMethods := map[string]bool{
		"cod":    true,
		"online": true,
		"wallet": true,
	}
	if !validMethods[paymentMethod] {
		utils.BadRequest(c, "Invalid payment method. Must be one of: cod, online, wallet", nil)
		return
	}

	// Check COD limit
	if paymentMethod == "cod" {
		cartDetails, err := utils.GetCartDetails(userID)
		if err != nil {
			utils.InternalServerError(c, "Failed to get cart details", err.Error())
			return
		}
		if cartDetails.FinalTotal > 1000 {
			utils.BadRequest(c, "Cash on Delivery is not available for orders above ₹1000. Please choose online payment or wallet payment.", nil)
			return
		}
	}

	db := config.DB
	var address models.Address
	if req.Address != nil {
		// Add new address
		newAddr := *req.Address
		newAddr.UserID = userID
		newAddr.IsDefault = false
		if err := db.Create(&newAddr).Error; err != nil {
			utils.InternalServerError(c, "Failed to create address", err.Error())
			return
		}
		address = newAddr
	} else if req.AddressID != 0 {
		db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address)
		if address.ID == 0 {
			utils.NotFound(c, "Address not found")
			return
		}
	} else {
		utils.BadRequest(c, "Provide either address_id or address object", nil)
		return
	}

	// Create order with transaction
	tx := db.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", nil)
		return
	}

	// Get cart details
	cartDetails, err := utils.GetCartDetails(userID)
	if err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to get cart details", err.Error())
		return
	}

	// Check if cart is empty
	if len(cartDetails.OrderItems) == 0 {
		tx.Rollback()
		utils.BadRequest(c, "Cannot place order with empty cart", nil)
		return
	}

	// Validate stock and reduce it for each item
	for _, item := range cartDetails.OrderItems {
		// Lock the book row for update
		var book models.Book
		if err := tx.Set("gorm:pessimistic_lock", true).First(&book, item.BookID).Error; err != nil {
			tx.Rollback()
			utils.NotFound(c, fmt.Sprintf("Book with ID %d not found", item.BookID))
			return
		}

		// Check if book has enough stock
		if book.Stock < item.Quantity {
			tx.Rollback()
			utils.BadRequest(c, fmt.Sprintf("Book '%s' does not have enough stock. Available: %d, Requested: %d", book.Name, book.Stock, item.Quantity), nil)
			return
		}

		// Reduce stock
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to update book stock", nil)
			return
		}
	}

	// Debug log cart details
	fmt.Printf("Cart Details - Subtotal: %.2f, Final Total: %.2f\n",
		cartDetails.Subtotal, cartDetails.FinalTotal)

	order := models.Order{
		UserID:         userID,
		AddressID:      address.ID,
		Address:        address,
		TotalAmount:    cartDetails.Subtotal,
		Discount:       cartDetails.ProductDiscount + cartDetails.CategoryDiscount,
		CouponDiscount: cartDetails.CouponDiscount,
		CouponCode:     cartDetails.CouponCode,
		FinalTotal:     cartDetails.FinalTotal,
		PaymentMethod:  paymentMethod,
		Status:         "Placed",
		OrderItems:     cartDetails.OrderItems,
	}

	// Debug log order details
	fmt.Printf("Order Details - Total Amount: %.2f, Final Total: %.2f\n",
		order.TotalAmount, order.FinalTotal)

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to create order", err.Error())
		return
	}

	// Clear cart for COD and wallet payments
	if paymentMethod == "cod" || paymentMethod == "wallet" {
		if err := tx.Where("user_id = ?", userID).Delete(&models.Cart{}).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to clear cart", err.Error())
			return
		}

		// Clear active coupon
		if err := tx.Where("user_id = ?", userID).Delete(&models.UserActiveCoupon{}).Error; err != nil {
			tx.Rollback()
			utils.InternalServerError(c, "Failed to clear active coupon", err.Error())
			return
		}
	}

	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit transaction", err.Error())
		return
	}

	// For online payment, return redirect URL
	if paymentMethod == "online" {
		utils.Success(c, "Please proceed to payment", gin.H{
			"status": "success",
			"data": gin.H{
				"redirect_url": fmt.Sprintf("/v1/user/checkout/payment/initiate?order_id=%d", order.ID),
				"order_id":     order.ID,
			},
		})
		return
	}

	// For COD and wallet payments, return order confirmation
	utils.Success(c, "Thank you for shopping with us! Your order has been placed successfully.", gin.H{
		"order_id":       order.ID,
		"payment_method": order.PaymentMethod,
		"status":         order.Status,
		"final_total":    cartDetails.FinalTotal, // Use exact cart final total
		"delivery_date":  "3-7 working days",
		"shipping_address": gin.H{
			"line1":       order.Address.Line1,
			"line2":       order.Address.Line2,
			"city":        order.Address.City,
			"state":       order.Address.State,
			"country":     order.Address.Country,
			"postal_code": order.Address.PostalCode,
		},
	})
}
