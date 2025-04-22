package controllers

import (
	"net/http"
	"strconv"

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
	Tax        float64 `json:"tax"`
	FinalTotal float64 `json:"final_total"`
}

func GetCheckoutSummary(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user in context"})
		return
	}
	userID := user.ID
	db := config.DB
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	var items []gin.H
	var subtotal, discountTotal float64
	for _, item := range cartItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil {
			continue
		}
		itemPrice := book.Price
		itemDiscount := 0.0
		if book.OriginalPrice > book.Price {
			itemDiscount = (book.OriginalPrice - book.Price) * float64(item.Quantity)
		}
		itemTotal := itemPrice * float64(item.Quantity)
		items = append(items, gin.H{
			"book_id":    book.ID,
			"name":       book.Name,
			"image_url":  book.ImageURL,
			"quantity":   item.Quantity,
			"price":      itemPrice,
			"discount":   itemDiscount,
			"item_total": itemTotal,
		})
		subtotal += itemTotal
		discountTotal += itemDiscount
	}
	tax := 0.05 * subtotal // 5% GST
	finalTotal := subtotal + tax - discountTotal
	c.JSON(http.StatusOK, CheckoutSummary{
		Items:      items,
		Subtotal:   subtotal,
		Discount:   discountTotal,
		Tax:        tax,
		FinalTotal: finalTotal,
	})
}

func PlaceOrder(c *gin.Context) {
	userVal, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user in context"})
		return
	}
	userID := user.ID
	var req struct {
		AddressID     uint            `json:"address_id"`
		Address       *models.Address `json:"address"`
		PaymentMethod string          `json:"payment_method" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.PaymentMethod != "COD" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Only Cash on Delivery is supported"})
		return
	}
	db := config.DB
	var address models.Address
	if req.Address != nil {
		// Add new address
		newAddr := *req.Address
		newAddr.UserID = userID
		newAddr.IsDefault = false // Don't override default here
		if err := db.Create(&newAddr).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create address"})
			return
		}
		address = newAddr
	} else if req.AddressID != 0 {
		db.Where("id = ? AND user_id = ?", req.AddressID, userID).First(&address)
		if address.ID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Address not found"})
			return
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provide either address_id or address object"})
		return
	}
	// Fetch cart items
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
		PaymentMethod: req.PaymentMethod,
		Status:        "Placed",
		OrderItems:    orderItems,
	}
	db.Create(&order)
	// Reduce stock for each book
	for _, item := range cartItems {
		db.Model(&models.Book{}).Where("id = ?", item.BookID).UpdateColumn("stock", gorm.Expr("stock - ?", item.Quantity))
	}
	// Clear cart
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	c.JSON(http.StatusOK, gin.H{
		"message":      "Order placed successfully",
		"order":        order,
		"redirect_url": "/thank-you?order_id=" + strconv.FormatUint(uint64(order.ID), 10),
		"thank_you_page": gin.H{
			"title":                 "Thank You for Your Order!",
			"subtitle":              "Your order has been placed and is being processed.",
			"order_id":              order.ID,
			"final_total":           order.FinalTotal,
			"payment_method":        order.PaymentMethod,
			"expected_delivery":     "3-7 business days",
			"continue_shopping_url": "/books",
		},
	})
}
