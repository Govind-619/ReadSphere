package routes

import (
	"net/http"

	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/middleware"
	"github.com/gin-gonic/gin"
)

// initUserRoutes initializes all user-related routes
func initUserRoutes(router *gin.RouterGroup) {
	// Public routes (no authentication required)
	// Page routes
	router.GET("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Login page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/signup", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Signup page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/verify-otp", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Verify OTP page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/forgot-password", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Forgot password page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/verify-reset-otp", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Verify reset OTP page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/reset-password", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Reset password page loaded successfully",
			"status":  "success",
		})
	})

	// API routes
	router.POST("/register", controllers.RegisterUser)
	router.POST("/login", controllers.LoginUser)
	router.POST("/verify-otp", controllers.VerifyOTP)
	router.POST("/forgot-password", controllers.ForgotPassword)
	router.POST("/verify-reset-otp", controllers.VerifyResetOTP)
	router.POST("/reset-password", controllers.ResetPassword)

	// Book routes
	router.GET("/books", controllers.GetBooks)
	router.GET("/books/:id", controllers.GetBookDetails)
	router.GET("/categories", controllers.ListCategories)
	router.GET("/categories/:id/books", controllers.ListBooksByCategory)

	// Protected routes (require authentication)
	protected := router.Group("/user")
	protected.Use(middleware.AuthMiddleware())
	{
		// Cart operations
		protected.POST("/cart/add", controllers.AddToCart)
		protected.GET("/cart", controllers.GetCart)
		protected.PUT("/cart/update", controllers.UpdateCart)
		protected.DELETE("/cart/remove", controllers.RemoveFromCart)
		protected.DELETE("/cart/clear", controllers.ClearCart)

		// Wishlist operations
		protected.POST("/wishlist/add", controllers.AddToWishlist)
		protected.GET("/wishlist", controllers.GetWishlist)
		protected.DELETE("/wishlist/remove", controllers.RemoveFromWishlist)

		// Checkout
		protected.GET("/checkout", controllers.GetCheckoutSummary)
		protected.POST("/checkout", controllers.PlaceOrder)

		// Orders
		protected.GET("/orders", controllers.ListOrders)
		protected.GET("/orders/:id", controllers.GetOrderDetails)
		protected.POST("/orders/:id/cancel", controllers.CancelOrder)
		protected.POST("/orders/:id/items/:item_id/cancel", controllers.CancelOrderItem)
		protected.POST("/orders/:id/return", controllers.ReturnOrder)
		protected.GET("/orders/:id/invoice", controllers.DownloadInvoice)

		// Reviews
		protected.POST("/books/:id/review", controllers.AddReview)
		protected.GET("/books/:id/reviews", controllers.GetBookReviews)
	}
}
