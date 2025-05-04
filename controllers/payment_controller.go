package controllers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	razorpay "github.com/razorpay/razorpay-go"
	"gorm.io/gorm"
)

// POST /user/checkout/payment/initiate
func InitiateRazorpayPayment(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	userID := user.ID

	var req struct {
		OrderID uint64 `json:"order_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. order_id is required", "details": err.Error()})
		return
	}
	db := config.DB
	var order models.Order
	db.Preload("Address").Where("id = ? AND user_id = ?", req.OrderID, userID).First(&order)
	if order.ID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order not found"})
		return
	}
	// Razorpay expects amount in paise. Only multiply by 100 if FinalTotal is in rupees (float).
	amountPaise := int(order.FinalTotal * 100)
	if order.FinalTotal > 1000 { // If FinalTotal is already in paise, do not multiply
		amountPaise = int(order.FinalTotal)
	}
	client := razorpay.NewClient(os.Getenv("RAZORPAY_KEY"), os.Getenv("RAZORPAY_SECRET"))
	orderData := map[string]interface{}{
		"amount":          amountPaise,
		"currency":        "INR",
		"receipt":         "order_rcptid_" + strconv.FormatUint(uint64(order.ID), 10),
		"payment_capture": 1,
	}
	rzOrder, err := client.Order.Create(orderData, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create Razorpay order", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"razorpay_order_id": rzOrder["id"],
		"amount":            rzOrder["amount"],
		"amount_display":    "₹" + fmt.Sprintf("%.2f", float64(amountPaise)/100),
		"currency":          rzOrder["currency"],
		"key":               os.Getenv("RAZORPAY_KEY"),
		"user": gin.H{
			"name":  user.Username,
			"email": user.Email,
		},
		"address":  order.Address,
		"order_id": order.ID,
	})
}

// POST /user/checkout/payment/verify
func VerifyRazorpayPayment(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)
	userID := user.ID
	var req struct {
		RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
		RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
		RazorpaySignature string `json:"razorpay_signature" binding:"required"`
		AddressID         uint   `json:"address_id"`
		CouponCode        string `json:"coupon_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	// Verify signature
	keySecret := os.Getenv("RAZORPAY_SECRET")
	data := req.RazorpayOrderID + "|" + req.RazorpayPaymentID
	h := hmac.New(sha256.New, []byte(keySecret))
	h.Write([]byte(data))
	generatedSignature := hex.EncodeToString(h.Sum(nil))
	if generatedSignature != req.RazorpaySignature {
		c.JSON(http.StatusBadRequest, gin.H{"status": "failure", "message": "Payment verification failed", "retry": true})
		return
	}
	// Payment is verified, create order in DB
	db := config.DB
	var address models.Address
	if req.AddressID != 0 {
		db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address)
		if address.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address not found"})
			return
		}
	} else {
		// If no address provided in request, try to get user's default address
		db.Where("user_id = ? AND is_default = ?", userID, true).First(&address)

		// If no default address, get any address for the user
		if address.ID == 0 {
			db.Where("user_id = ?", userID).First(&address)
		}

		// If user has no addresses at all, return error
		if address.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No address found. Please add an address before completing payment."})
			return
		}
	}
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	if len(cartItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}
	var orderItems []models.OrderItem
	var subtotal, discountTotal float64
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil || book.Stock < item.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available or insufficient stock for book_id " + strconv.FormatUint(uint64(item.BookID), 10)})
			return
		}
		itemPrice := book.Price
		itemDiscount := 0.0
		if book.OriginalPrice > book.Price {
			itemDiscount = (book.OriginalPrice - book.Price) * float64(item.Quantity)
		}
		itemTotal := itemPrice * float64(item.Quantity)
		orderItems = append(orderItems, models.OrderItem{
			BookID:   book.ID,
			Book:     *book,
			Quantity: item.Quantity,
			Price:    itemPrice,
			Discount: itemDiscount,
			Total:    itemTotal,
		})
		subtotal += itemTotal
		discountTotal += itemDiscount
	}
	tax := 0.05 * subtotal
	finalTotal := subtotal + tax - discountTotal
	order := models.Order{
		UserID:        userID,
		AddressID:     address.ID,
		Address:       address,
		TotalAmount:   subtotal,
		Discount:      discountTotal,
		Tax:           tax,
		FinalTotal:    finalTotal,
		PaymentMethod: "RAZORPAY",
		Status:        "Placed",
		OrderItems:    orderItems,
	}

	// Create order with transaction to ensure atomic operation
	tx := db.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to begin transaction"})
		return
	}

	if err := tx.Create(&order).Error; err != nil {
		tx.Rollback()
		// Log the error for debugging
		fmt.Printf("Error creating order: %v\n", err)
		fmt.Printf("Address ID: %d, User ID: %d\n", address.ID, userID)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":      "Failed to create order",
			"details":    err.Error(),
			"address_id": address.ID,
		})
		return
	}

	// Reduce stock within the same transaction
	for _, item := range cartItems {
		if err := tx.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity)).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update book stock"})
			return
		}
	}

	// Clear cart after successful online payment verification
	if err := tx.Where("user_id = ?", userID).Delete(&models.Cart{}).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear cart"})
		return
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Fetch the complete order with all relationships
	var completeOrder models.Order
	if err := db.Preload("OrderItems.Book").Preload("Address").Preload("User").First(&completeOrder, order.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Payment was processed but failed to load order details. Please check your orders page."})
		return
	}

	// Prepare response
	c.JSON(http.StatusOK, gin.H{
		"status":                "success",
		"message":               "Thank you for your payment! Your order has been placed.",
		"order_id":              completeOrder.ID,
		"final_total":           completeOrder.FinalTotal,
		"payment_method":        completeOrder.PaymentMethod,
		"order_details_url":     "/user/orders/" + strconv.FormatUint(uint64(completeOrder.ID), 10),
		"continue_shopping_url": "/books",
	})
}

// GetPaymentMethods returns all available payment methods for checkout
func GetPaymentMethods(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user := userVal.(models.User)

	// Get or create wallet
	wallet, err := getOrCreateWallet(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get wallet"})
		return
	}

	// Fetch final total from query parameters or request to check if wallet has enough balance
	finalTotalStr := c.Query("total")
	var finalTotal float64
	if finalTotalStr != "" {
		finalTotal, _ = strconv.ParseFloat(finalTotalStr, 64)
	}

	// Available payment methods
	paymentMethods := []gin.H{
		{
			"id":          "cod",
			"name":        "Cash on Delivery",
			"description": "Pay when you receive your order",
			"available":   true,
		},
		{
			"id":          "online",
			"name":        "Online Payment",
			"description": "Pay securely with Razorpay",
			"available":   true,
		},
		{
			"id":           "wallet",
			"name":         "Wallet",
			"description":  fmt.Sprintf("Pay using your wallet balance (₹%.2f available)", wallet.Balance),
			"available":    finalTotal == 0 || wallet.Balance >= finalTotal,
			"balance":      fmt.Sprintf("%.2f", wallet.Balance),
			"insufficient": finalTotal > 0 && wallet.Balance < finalTotal,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"payment_methods": paymentMethods,
		"wallet_balance":  fmt.Sprintf("%.2f", wallet.Balance),
	})
}
