package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/gin-gonic/gin"
)

func RegisterBookImageRoutes(router *gin.RouterGroup) {
	router.POST("/books/:id/images", controllers.UploadBookImages)
	router.GET("/books/:id/images", controllers.GetBookImages)
}
