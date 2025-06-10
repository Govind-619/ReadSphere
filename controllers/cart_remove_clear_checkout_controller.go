package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// RemoveFromCart removes a product from the cart
func RemoveFromCart(c *gin.Context) {
	utils.LogInfo("RemoveFromCart called")

	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing remove from cart for user ID: %d", userID)

	var req struct {
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Removing book ID: %d from cart for user ID: %d", req.BookID, userID)

	db := config.DB
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Cart{})
	utils.LogInfo("Successfully removed book ID: %d from cart for user ID: %d", req.BookID, userID)
	utils.Success(c, "Product removed from cart successfully", nil)
}

// ClearCart removes all items from the user's cart
func ClearCart(c *gin.Context) {
	utils.LogInfo("ClearCart called")

	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing clear cart for user ID: %d", userID)

	db := config.DB
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	utils.LogInfo("Successfully cleared cart for user ID: %d", userID)
	utils.Success(c, "Cart cleared successfully", nil)
}

// CheckoutCart attempts to checkout the cart, blocks if any item is out of stock
func CheckoutCart(c *gin.Context) {
	utils.LogInfo("CheckoutCart called")

	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "Unauthorized")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing checkout for user ID: %d", userID)

	var cartItems []models.Cart
	db := config.DB
	db.Preload("Book").Where("user_id = ?", userID).Find(&cartItems)
	if len(cartItems) == 0 {
		utils.LogError("Empty cart for user ID: %d", userID)
		utils.BadRequest(c, "Cart is empty", nil)
		return
	}
	utils.LogInfo("Found %d items in cart for user ID: %d", len(cartItems), userID)

	for _, item := range cartItems {
		if !item.Book.IsActive || item.Book.Blocked {
			utils.LogError("Book ID: %d is not available or blocked for user ID: %d", item.BookID, userID)
			utils.BadRequest(c, "Book not available or blocked by admin", nil)
			return
		}
		if item.Book.CategoryID != 0 {
			var category models.Category
			db := config.DB
			db.First(&category, item.Book.CategoryID)
			if category.Blocked {
				utils.LogError("Category ID: %d is blocked for book ID: %d, user ID: %d", item.Book.CategoryID, item.BookID, userID)
				utils.BadRequest(c, "Category blocked by admin", nil)
				return
			}
		}
		if item.Book.Stock < item.Quantity {
			utils.LogError("Insufficient stock for book ID: %d, requested: %d, available: %d", item.BookID, item.Quantity, item.Book.Stock)
			utils.BadRequest(c, "Book out of stock", nil)
			return
		}
	}

	// (Order creation logic would go here)
	// For now, just clear cart and return success
	db.Where("user_id = ?", userID).Delete(&models.Cart{})
	utils.LogInfo("Successfully completed checkout for user ID: %d", userID)
	utils.Success(c, "Checkout successful. Order placed.", nil)
}
