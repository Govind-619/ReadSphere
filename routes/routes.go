package routes

import (
	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// SetupRouter initializes and returns the Gin router with all routes
func SetupRouter() *gin.Engine {
	router := gin.Default()

	// Setup session middleware with a secure key
	store := cookie.NewStore([]byte("your-secret-key"))
	store.Options(sessions.Options{
		MaxAge:   60 * 60 * 24, // 1 day
		Path:     "/",
		Secure:   false, // Set to true in production with HTTPS
		HttpOnly: true,
	})
	router.Use(sessions.Sessions("readsphere", store))

	// Root route for health check or info
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Read Sphere backend is running...!",
		})
	})

	// Auth routes (for OAuth)
	auth := router.Group("/auth")
	{
		auth.GET("/google/login", controllers.GoogleLogin)
		auth.GET("/google/callback", controllers.GoogleCallback)

		// Temporary test route to view all users
		auth.GET("/test/users", func(c *gin.Context) {
			var users []models.User
			if err := config.DB.Order("created_at DESC").Find(&users).Error; err != nil {
				utils.LogError("Failed to fetch users: %v", err)
				utils.InternalServerError(c, "Failed to fetch users", err.Error())
				return
			}

			// Create a slice to hold only necessary user details
			var userDetails []gin.H
			for _, user := range users {
				// Log each user's details for debugging
				utils.LogInfo("User found - ID: %d, Email: %s, GoogleID: %s, CreatedAt: %v",
					user.ID, user.Email, user.GoogleID, user.CreatedAt)

				userDetails = append(userDetails, gin.H{
					"id":          user.ID,
					"username":    user.Username,
					"email":       user.Email,
					"first_name":  user.FirstName,
					"last_name":   user.LastName,
					"is_verified": user.IsVerified,
					"is_blocked":  user.IsBlocked,
					"created_at":  user.CreatedAt,
					"google_id":   user.GoogleID,
					"last_login":  user.LastLoginAt,
				})
			}

			// Log total count and Google users count
			googleUsers := 0
			for _, user := range users {
				if user.GoogleID != "" {
					googleUsers++
				}
			}

			utils.LogInfo("Users retrieved successfully - Total: %d, Google Users: %d", len(users), googleUsers)
			utils.Success(c, "Users retrieved successfully", gin.H{
				"users":              userDetails,
				"count":              len(users),
				"google_users_count": googleUsers,
			})
		})
	}

	// API version group
	api := router.Group("/v1")
	{
		// Initialize user routes (includes regular auth routes)
		initUserRoutes(api)

		// Initialize admin routes
		initAdminRoutes(api)

		// Initialize user profile routes
		SetupUserProfileRoutes(router)
	}

	utils.LogInfo("Routes setup completed")
	return router
}
