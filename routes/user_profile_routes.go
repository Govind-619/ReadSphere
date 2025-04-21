package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/middleware"
	"github.com/gin-gonic/gin"
)

// SetupUserProfileRoutes sets up the routes for user profile management
func SetupUserProfileRoutes(router *gin.Engine) {
	// User profile routes (protected)
	profile := router.Group("/v1/profile")
	profile.Use(middleware.AuthMiddleware())
	{
		// Get user profile
		profile.GET("", controllers.GetUserProfile)

		// Update profile (excluding email)
		profile.PUT("", controllers.UpdateProfile)

		// Update email (with OTP verification)
		profile.PUT("/email", controllers.UpdateEmail)
		profile.POST("/email/verify", controllers.VerifyEmailUpdate)

		// Change password
		profile.PUT("/password", controllers.ChangePassword)

		// Upload profile image
		profile.POST("/image", controllers.UploadProfileImage)

		// Address management
		profile.POST("/address", controllers.AddAddress)
		profile.PUT("/address/:id", controllers.EditAddress)
		profile.DELETE("/address/:id", controllers.DeleteAddress)
		profile.PUT("/address/:id/default", controllers.SetDefaultAddress)
		profile.GET("/address", controllers.GetAddresses)
	}
}
