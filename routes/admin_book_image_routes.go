package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

func RegisterAdminBookImageRoutes(router *gin.RouterGroup) {
	utils.LogInfo("Registering admin book image routes")

	admin := router.Group("/admin")
	admin.POST("/books/:id/images", controllers.UploadBookImages)
	admin.GET("/books/:id/images", controllers.GetBookImages)

	utils.LogInfo("Admin book image routes registration completed")
}
