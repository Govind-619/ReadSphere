package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AddToWishlist adds a product to the user's wishlist
func AddToWishlist(c *gin.Context) {
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
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	// Check if already in wishlist
	db := config.DB
	var wishlist models.Wishlist
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&wishlist)
	if wishlist.ID != 0 {
		utils.Success(c, "Product already in wishlist", nil)
		return
	}

	// Check if book exists and is active
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		utils.NotFound(c, "Book not found")
		return
	}
	if !book.IsActive {
		utils.BadRequest(c, "Book not available", nil)
		return
	}

	// Add to wishlist
	newWishlist := models.Wishlist{
		UserID: userID,
		BookID: req.BookID,
	}
	if err := db.Create(&newWishlist).Error; err != nil {
		utils.InternalServerError(c, "Failed to add to wishlist", err.Error())
		return
	}

	// Build response (wishlist summary)
	wishlistItems := getWishlistItems(userID)
	utils.Success(c, "Product added to wishlist successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// GetWishlist retrieves the user's wishlist
func GetWishlist(c *gin.Context) {
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

	wishlistItems := getWishlistItems(userID)
	utils.Success(c, "Wishlist retrieved successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// RemoveFromWishlist removes a product from the wishlist
func RemoveFromWishlist(c *gin.Context) {
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
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}

	db := config.DB
	result := db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Wishlist{})
	if result.Error != nil {
		utils.InternalServerError(c, "Failed to remove from wishlist", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Item not found in wishlist")
		return
	}

	// Build response (wishlist summary)
	wishlistItems := getWishlistItems(userID)
	utils.Success(c, "Product removed from wishlist successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// Helper function to get wishlist items
func getWishlistItems(userID uint) []gin.H {
	db := config.DB
	var wishlistItems []models.Wishlist
	db.Where("user_id = ?", userID).Find(&wishlistItems)

	var minimalWishlistItems []gin.H
	for _, item := range wishlistItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil {
			continue
		}
		minimalWishlistItems = append(minimalWishlistItems, gin.H{
			"book_id":   book.ID,
			"name":      book.Name,
			"image_url": book.ImageURL,
			"price":     book.Price,
			"stock_status": func() string {
				if book.Stock < 1 {
					return "Out of Stock"
				}
				if book.Stock <= 3 {
					return "Only a few left"
				}
				return "In Stock"
			}(),
		})
	}
	return minimalWishlistItems
}
