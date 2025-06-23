package routes

import (
	"net/http"

	"github.com/Govind-619/ReadSphere/controllers"
	paymentcontroller "github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/middleware"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// initUserRoutes initializes all user-related routes
func initUserRoutes(router *gin.RouterGroup) {
	utils.LogInfo("Initializing user routes")

	// Public routes (no authentication required)
	// Page routes
	router.GET("/login", func(c *gin.Context) {
		utils.LogInfo("Login page accessed")
		c.JSON(http.StatusOK, gin.H{
			"message": "Login page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/signup", func(c *gin.Context) {
		utils.LogInfo("Signup page accessed")
		c.JSON(http.StatusOK, gin.H{
			"message": "Signup page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/verify-otp", func(c *gin.Context) {
		utils.LogInfo("Verify OTP page accessed")
		c.JSON(http.StatusOK, gin.H{
			"message": "Verify OTP page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/forgot-password", func(c *gin.Context) {
		utils.LogInfo("Forgot password page accessed")
		c.JSON(http.StatusOK, gin.H{
			"message": "Forgot password page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/verify-reset-otp", func(c *gin.Context) {
		utils.LogInfo("Verify reset OTP page accessed")
		c.JSON(http.StatusOK, gin.H{
			"message": "Verify reset OTP page loaded successfully",
			"status":  "success",
		})
	})

	router.GET("/reset-password", func(c *gin.Context) {
		utils.LogInfo("Reset password page accessed")
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

	// Referral routes
	router.GET("/referral/:code", controllers.GetReferralCodeInfo)

	// Protected routes (require authentication)
	// Referral: Accept referral via token (public, user must sign up and then visit link)
	protected := router.Group("/user")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.POST("/checkout/payment/initiate", paymentcontroller.InitiateRazorpayPayment)
		protected.POST("/checkout/payment/verify", paymentcontroller.VerifyRazorpayPayment)
		protected.GET("/checkout/payment/methods", paymentcontroller.GetPaymentMethods)
		// Test payment simulation (only in development)
		protected.GET("/checkout/payment/simulate", paymentcontroller.SimulatePayment)

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
		protected.POST("/orders/:id/items/:item_id/return", controllers.ReturnOrderItem)
		protected.GET("/orders/:id/invoice", controllers.DownloadInvoice)

		// Logout
		protected.POST("/logout", controllers.UserLogout)

		// Reviews
		protected.POST("/books/:id/review", controllers.AddReview)
		protected.GET("/books/:id/reviews", controllers.GetBookReviews)

		// Coupon routes
		protected.POST("/coupons/apply", controllers.ApplyCoupon)
		protected.POST("/coupons/remove", controllers.RemoveCoupon)
		protected.GET("/coupons", controllers.GetCoupons)

		// Wallet routes
		protected.GET("/wallet", controllers.GetWalletBalance)
		protected.GET("/wallet/transactions", controllers.GetWalletTransactions)
		protected.POST("/wallet/topup/initiate", controllers.InitiateWalletTopup)
		protected.POST("/wallet/topup/verify", controllers.VerifyWalletTopup)
		// Test wallet topup payment simulation (only in development)
		protected.GET("/wallet/topup/simulate", controllers.SimulateWalletTopupPayment)

		// User referral routes
		protected.GET("/referral/code", controllers.GetUserReferralCode)
		protected.GET("/referral/list", controllers.GetUserReferrals)
	}

	utils.LogInfo("User routes initialization completed")
}
