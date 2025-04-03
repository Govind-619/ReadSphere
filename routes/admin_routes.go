package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/middleware"
	"github.com/gin-gonic/gin"
)

// initAdminRoutes initializes all admin-related routes
func initAdminRoutes(router *gin.RouterGroup) {
	admin := router.Group("/admin")
	{
		// Public admin routes
		admin.POST("/login", controllers.AdminLogin)
		admin.POST("/logout", controllers.AdminLogout)

		// Protected admin routes
		admin.Use(middleware.AuthMiddleware(), middleware.AdminMiddleware())
		{
			// User management
			admin.GET("/users", controllers.GetUsers)
			admin.PATCH("/users/:id/block", controllers.BlockUser)
			admin.PATCH("/users/:id/unblock", controllers.UnblockUser)

			// Category management
			admin.POST("/categories", controllers.CreateCategory)
			admin.GET("/categories", controllers.GetCategories)
			admin.PUT("/categories/:id", controllers.UpdateCategory)
			admin.DELETE("/categories/:id", controllers.DeleteCategory)

			// Product management
			admin.POST("/products", controllers.CreateProduct)
			admin.GET("/products", controllers.GetProducts)
			admin.PUT("/products/:id", controllers.UpdateProduct)
			admin.DELETE("/products/:id", controllers.DeleteProduct)
		}
	}
}
