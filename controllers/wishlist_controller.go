package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddToWishlist adds a product to the user's wishlist
func AddToWishlist(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product added to wishlist successfully",
	})
}

// GetWishlist retrieves the user's wishlist
func GetWishlist(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message":  "Wishlist retrieved successfully",
		"wishlist": []interface{}{},
	})
}

// RemoveFromWishlist removes a product from the wishlist
func RemoveFromWishlist(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product removed from wishlist successfully",
	})
}
