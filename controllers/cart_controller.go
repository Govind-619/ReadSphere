package controllers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AddToCart adds a product to the user's cart
func AddToCart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product added to cart successfully",
	})
}

// GetCart retrieves the user's cart
func GetCart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Cart retrieved successfully",
		"cart":    []interface{}{},
	})
}

// UpdateCart updates the quantity of items in the cart
func UpdateCart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Cart updated successfully",
	})
}

// RemoveFromCart removes a product from the cart
func RemoveFromCart(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product removed from cart successfully",
	})
}
