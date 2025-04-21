package controllers

import (
	"net/http"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AddToCart adds a product to the user's cart with validation
func AddToCart(c *gin.Context) {
	// Assume user ID is set in context by auth middleware
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
		BookID   uint `json:"book_id" binding:"required"`
		Quantity int  `json:"quantity"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	if req.Quantity < 1 {
		req.Quantity = 1
	}
	const maxQuantity = 5
	if req.Quantity > maxQuantity {
		req.Quantity = maxQuantity
	}

	// Fetch book using GetBookByIDForCart to avoid Images scan error
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}
	if !book.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available"})
		return
	}
	if book.Stock < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book out of stock"})
		return
	}

	// Remove from wishlist if present
	var wishlist models.Wishlist
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&wishlist)
	if wishlist.ID != 0 {
		db.Delete(&wishlist)
	}

	// Check if already in cart
	var cart models.Cart
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&cart)
	if cart.ID != 0 {
		// Increment quantity
		newQty := cart.Quantity + req.Quantity
		if newQty > maxQuantity {
			newQty = maxQuantity
		}
		if newQty > book.Stock {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book out of stock"})
			return
		}
		cart.Quantity = newQty
		db.Save(&cart)
		c.JSON(http.StatusOK, gin.H{"message": "Cart updated (incremented)", "cart": cart})
		return
	}
	if req.Quantity > book.Stock {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book out of stock"})
		return
	}
	newCart := models.Cart{
		UserID:   userID,
		BookID:   req.BookID,
		Quantity: req.Quantity,
	}
	db.Create(&newCart)
	// Fetch Book details for response using GetBookByIDForCart (avoids Images scan error)
	book, _ = utils.GetBookByIDForCart(req.BookID)
	newCart.Book = *book
	// After adding, fetch all cart items for the user
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Stock < cartItems[i].Quantity {
			canCheckout = false
		}
	}
	var minimalCartItems []gin.H
	subtotal := 0.0
	for _, item := range cartItems {
		book := item.Book
		itemSubtotal := book.Price * float64(item.Quantity)
		subtotal += itemSubtotal
		minimalCartItems = append(minimalCartItems, gin.H{
			"book_id":   book.ID,
			"name":      book.Name,
			"image_url": book.ImageURL,
			"price":     book.Price,
			"quantity":  item.Quantity,
			"subtotal":  itemSubtotal,
			"stock_status": func() string {
				if book.Stock < item.Quantity {
					return "Out of Stock"
				}
				if book.Stock <= 3 {
					return "Only a few left"
				}
				return "In Stock"
			}(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "Product added to cart",
		"cart":         minimalCartItems,
		"subtotal":     subtotal,
		"total":        subtotal,
		"can_checkout": canCheckout,
	})
}

// GetCart retrieves the user's cart and blocks checkout if any item is out of stock
func GetCart(c *gin.Context) {
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
	var cartItems []models.Cart
	db := config.DB
	db.Where("user_id = ?", userID).Find(&cartItems)
	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Stock < cartItems[i].Quantity {
			canCheckout = false
		}
	}
	var minimalCartItems []gin.H
	subtotal := 0.0
	for _, item := range cartItems {
		book := item.Book
		itemSubtotal := book.Price * float64(item.Quantity)
		subtotal += itemSubtotal
		minimalCartItems = append(minimalCartItems, gin.H{
			"book_id":   book.ID,
			"name":      book.Name,
			"image_url": book.ImageURL,
			"price":     book.Price,
			"quantity":  item.Quantity,
			"subtotal":  itemSubtotal,
			"stock_status": func() string {
				if book.Stock < item.Quantity {
					return "Out of Stock"
				}
				if book.Stock <= 3 {
					return "Only a few left"
				}
				return "In Stock"
			}(),
		})
	}
	if minimalCartItems == nil {
		minimalCartItems = []gin.H{}
	}
	c.JSON(http.StatusOK, gin.H{
		"cart":         minimalCartItems,
		"subtotal":     subtotal,
		"total":        subtotal, // Add discounts, shipping, taxes if/when needed
		"can_checkout": canCheckout,
	})
}

// UpdateCart updates the quantity of items in the cart (increment/decrement)
func UpdateCart(c *gin.Context) {
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
		BookID uint   `json:"book_id" binding:"required"`
		Action string `json:"action" binding:"required"` // "increment" or "decrement"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	const maxQuantity = 5
	var cart models.Cart
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&cart)
	if cart.ID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cart item not found"})
		return
	}
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}
	if req.Action == "increment" {
		if cart.Quantity >= maxQuantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Max quantity reached"})
			return
		}
		if cart.Quantity+1 > book.Stock {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book out of stock"})
			return
		}
		cart.Quantity++
		db.Save(&cart)
	} else if req.Action == "decrement" {
		if cart.Quantity <= 1 {
			db.Delete(&cart)
		} else {
			cart.Quantity--
			db.Save(&cart)
		}
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
		return
	}
	// After update, return full cart summary
	var cartItems []models.Cart
	db.Where("user_id = ?", userID).Find(&cartItems)
	canCheckout := true
	for i := range cartItems {
		book, err := utils.GetBookByIDForCart(cartItems[i].BookID)
		if err == nil && book != nil {
			cartItems[i].Book = *book
		}
		if !cartItems[i].Book.IsActive || cartItems[i].Book.Stock < cartItems[i].Quantity {
			canCheckout = false
		}
	}
	var minimalCartItems []gin.H
	subtotal := 0.0
	for _, item := range cartItems {
		book := item.Book
		itemSubtotal := book.Price * float64(item.Quantity)
		subtotal += itemSubtotal
		minimalCartItems = append(minimalCartItems, gin.H{
			"book_id":   book.ID,
			"name":      book.Name,
			"image_url": book.ImageURL,
			"price":     book.Price,
			"quantity":  item.Quantity,
			"subtotal":  itemSubtotal,
			"stock_status": func() string {
				if book.Stock < item.Quantity {
					return "Out of Stock"
				}
				if book.Stock <= 3 {
					return "Only a few left"
				}
				return "In Stock"
			}(),
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"message":      "Cart updated",
		"cart":         minimalCartItems,
		"subtotal":     subtotal,
		"total":        subtotal,
		"can_checkout": canCheckout,
	})
}

// ClearCart removes all items from the user's cart
func ClearCart(c *gin.Context) {
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
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	c.JSON(http.StatusOK, gin.H{"message": "Cart cleared successfully"})
}

// CheckoutCart attempts to checkout the cart, blocks if any item is out of stock
func CheckoutCart(c *gin.Context) {
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
	var cartItems []models.Cart
	db := config.DB
	db.Preload("Book").Where("user_id = ?", userID).Find(&cartItems)
	if len(cartItems) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cart is empty"})
		return
	}
	for _, item := range cartItems {
		if !item.Book.IsActive {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available"})
			return
		}
		if item.Book.Stock < item.Quantity {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Book out of stock"})
			return
		}
	}
	// (Order creation logic would go here)
	// For now, just clear cart and return success
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	c.JSON(http.StatusOK, gin.H{"message": "Checkout successful. Order placed."})
}

// RemoveFromCart removes a product from the cart
func RemoveFromCart(c *gin.Context) {
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
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Cart{})
	c.JSON(http.StatusOK, gin.H{"message": "Product removed from cart successfully"})
}
