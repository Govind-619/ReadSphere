package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/gin-gonic/gin"
)

func RegisterAdminBookImageRoutes(router *gin.RouterGroup) {
	admin := router.Group("/admin")
	admin.POST("/books/:id/images", controllers.UploadBookImages)
	admin.GET("/books/:id/images", controllers.GetBookImages)
}
