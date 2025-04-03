package routes

import (
	"net/http"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/models"
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

	// Auth routes (for OAuth)
	auth := router.Group("/auth")
	{
		auth.GET("/google/login", controllers.GoogleLogin)
		auth.GET("/google/callback", controllers.GoogleCallback)

		// Temporary test route to view all users
		auth.GET("/test/users", func(c *gin.Context) {
			var users []models.User
			if err := config.DB.Find(&users).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, users)
		})
	}

	// API version group
	api := router.Group("/v1")
	{
		// Initialize user routes (includes regular auth routes)
		initUserRoutes(api)

		// Initialize admin routes
		initAdminRoutes(api)
	}

	return router
}
