package controllers

import (
	"net/http"
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AddToWishlist adds a product to the user's wishlist
func AddToWishlist(c *gin.Context) {
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

	// Check if already in wishlist
	db := config.DB
	var wishlist models.Wishlist
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).First(&wishlist)
	if wishlist.ID != 0 {
		c.JSON(http.StatusOK, gin.H{"message": "Product already in wishlist"})
		return
	}

	// Check if book exists and is active
	book, err := utils.GetBookByIDForCart(req.BookID)
	if err != nil || book == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Book not found"})
		return
	}
	if !book.IsActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Book not available"})
		return
	}

	// Add to wishlist
	newWishlist := models.Wishlist{
		UserID: userID,
		BookID: req.BookID,
	}
	db.Create(&newWishlist)

	// Build response (wishlist summary)
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
	c.JSON(http.StatusOK, gin.H{
		"message":  "Product added to wishlist successfully",
		"wishlist": minimalWishlistItems,
	})
}

// GetWishlist retrieves the user's wishlist
func GetWishlist(c *gin.Context) {
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
	c.JSON(http.StatusOK, gin.H{
		"wishlist": minimalWishlistItems,
	})
}

// RemoveFromWishlist removes a product from the wishlist
func RemoveFromWishlist(c *gin.Context) {
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
	db.Where("user_id = ? AND book_id = ?", userID, req.BookID).Delete(&models.Wishlist{})
	// Build response (wishlist summary)
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
	c.JSON(http.StatusOK, gin.H{
		"message":  "Product removed from wishlist successfully",
		"wishlist": minimalWishlistItems,
	})
}

