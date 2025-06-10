package controllers

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AddToWishlist adds a product to the user's wishlist
func AddToWishlist(c *gin.Context) {
	utils.LogInfo("AddToWishlist called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing wishlist addition for user ID: %d", userID)

	var req struct {
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Adding book ID: %d to wishlist for user ID: %d", req.BookID, userID)

	// Check if already in wishlist
	db := config.DB
	var wishlist models.Wishlist
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&wishlist)
	if wishlist.ID != 0 {
		utils.LogInfo("Book ID: %d already in wishlist for user ID: %d", req.BookID, userID)
		utils.Success(c, "Product already in wishlist", nil)
		return
	}

	// Check if book exists and is active
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		utils.LogError("Book not found - Book ID: %d for user ID: %d", req.BookID, userID)
		utils.NotFound(c, "Book not found")
		return
	}
	if !book.IsActive {
		utils.LogError("Book not available - Book ID: %d for user ID: %d", req.BookID, userID)
		utils.BadRequest(c, "Book not available", nil)
		return
	}

	// Add to wishlist
	newWishlist := models.Wishlist{
		UserID: userID,
		BookID: req.BookID,
	}
	if err := db.Create(&newWishlist).Error; err != nil {
		utils.LogError("Failed to add to wishlist - Book ID: %d, User ID: %d: %v", req.BookID, userID, err)
		utils.InternalServerError(c, "Failed to add to wishlist", err.Error())
		return
	}
	utils.LogInfo("Successfully added book ID: %d to wishlist for user ID: %d", req.BookID, userID)

	// Build response (wishlist summary)
	wishlistItems := getWishlistItems(userID)
	utils.Success(c, "Product added to wishlist successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// GetWishlist retrieves the user's wishlist
func GetWishlist(c *gin.Context) {
	utils.LogInfo("GetWishlist called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Retrieving wishlist for user ID: %d", userID)

	wishlistItems := getWishlistItems(userID)
	utils.LogInfo("Successfully retrieved %d items from wishlist for user ID: %d", len(wishlistItems), userID)
	utils.Success(c, "Wishlist retrieved successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// RemoveFromWishlist removes a product from the wishlist
func RemoveFromWishlist(c *gin.Context) {
	utils.LogInfo("RemoveFromWishlist called")
	userVal, exists := c.Get("user")
	if !exists {
		utils.LogError("User not found in context")
		utils.Unauthorized(c, "User not found")
		return
	}
	user, ok := userVal.(models.User)
	if !ok {
		utils.LogError("Invalid user type in context")
		utils.BadRequest(c, "Invalid user in context", nil)
		return
	}
	userID := user.ID
	utils.LogInfo("Processing wishlist removal for user ID: %d", userID)

	var req struct {
		BookID uint `json:"book_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid request format for user ID: %d: %v", userID, err)
		utils.BadRequest(c, "Invalid request", err.Error())
		return
	}
	utils.LogInfo("Removing book ID: %d from wishlist for user ID: %d", req.BookID, userID)

	db := config.DB
	result := db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Wishlist{})
	if result.Error != nil {
		utils.LogError("Failed to remove from wishlist - Book ID: %d, User ID: %d: %v", req.BookID, userID, result.Error)
		utils.InternalServerError(c, "Failed to remove from wishlist", result.Error.Error())
		return
	}
	if result.RowsAffected == 0 {
		utils.LogError("Item not found in wishlist - Book ID: %d, User ID: %d", req.BookID, userID)
		utils.NotFound(c, "Item not found in wishlist")
		return
	}
	utils.LogInfo("Successfully removed book ID: %d from wishlist for user ID: %d", req.BookID, userID)

	// Build response (wishlist summary)
	wishlistItems := getWishlistItems(userID)
	utils.Success(c, "Product removed from wishlist successfully", gin.H{
		"wishlist": wishlistItems,
	})
}

// Helper function to get wishlist items
func getWishlistItems(userID uint) []gin.H {
	utils.LogDebug("Getting wishlist items for user ID: %d", userID)
	db := config.DB
	var wishlistItems []models.Wishlist
	db.Where("user_id = ?", userID).Find(&wishlistItems)
	utils.LogDebug("Found %d items in wishlist for user ID: %d", len(wishlistItems), userID)

	var minimalWishlistItems []gin.H
	for _, item := range wishlistItems {
		book, err := utils.GetBookByIDForCart(item.BookID)
		if err != nil || book == nil {
			utils.LogError("Failed to get book details - Book ID: %d for user ID: %d: %v", item.BookID, userID, err)
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
	utils.LogDebug("Processed %d wishlist items for user ID: %d", len(minimalWishlistItems), userID)
	return minimalWishlistItems
}
