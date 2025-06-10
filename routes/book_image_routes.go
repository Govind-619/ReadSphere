package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

func RegisterBookImageRoutes(router *gin.RouterGroup) {
	utils.LogInfo("Registering book image routes")

	router.POST("/books/:id/images", controllers.UploadBookImages)
	router.GET("/books/:id/images", controllers.GetBookImages)

	utils.LogInfo("Book image routes registration completed")
}
