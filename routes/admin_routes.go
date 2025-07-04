package routes

import (
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/middleware"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// initAdminRoutes initializes all admin-related routes
func initAdminRoutes(router *gin.RouterGroup) {
	utils.LogInfo("Registering admin routes")

	admin := router.Group("/admin")
	{
		// Public admin routes
		admin.GET("/login", func(c *gin.Context) {
			utils.LogInfo("Admin login page loaded successfully")
			c.JSON(200, gin.H{
				"message": "Admin login page loaded successfully",
				"status":  "success",
			})
		})
		admin.POST("/login", controllers.AdminLogin)

		// Protected admin routes
		admin.Use(middleware.AdminAuthMiddleware())
		{
			// Logout (must be authenticated)
			admin.POST("/logout", controllers.AdminLogout)

			// Dashboard
			admin.GET("/dashboard", controllers.GetDashboardOverview)

			// User management
			admin.GET("/users", controllers.GetUsers)
			admin.PUT("/users/:id/block", controllers.BlockUser)

			// Category management
			admin.GET("/categories", controllers.GetCategories)
			admin.POST("/categories", controllers.CreateCategory)
			admin.PUT("/categories/:id", controllers.UpdateCategory)
			admin.DELETE("/categories/:id", controllers.DeleteCategory)
			admin.GET("/categories/:id/books", controllers.ListBooksByCategory)
			admin.PATCH("/categories/:id/block", controllers.ToggleCategoryBlock)

			// Book management
			admin.GET("/books", controllers.GetBooks)
			admin.POST("/books", controllers.CreateBook)
			admin.PUT("/books/field/:field/:value", controllers.UpdateBookByField)
			admin.GET("/books/:id", controllers.GetBookDetails)
			admin.PUT("/books/:id", controllers.UpdateBook)
			admin.DELETE("/books/:id", controllers.DeleteBook)
			admin.GET("/books/:id/check", controllers.CheckBookExists)
			admin.GET("/books/:id/reviews", controllers.GetBookReviews)
			admin.PUT("/books/:id/reviews/:reviewId/approve", controllers.ApproveReview)
			admin.DELETE("/books/:id/reviews/:reviewId", controllers.DeleteReview)

			// Genre management routes
			admin.POST("/genres", controllers.CreateGenre)
			admin.PUT("/genres/:id", controllers.UpdateGenre)
			admin.DELETE("/genres/:id", controllers.DeleteGenre)
			admin.GET("/genres", controllers.GetGenres)
			admin.GET("/genres/:id", controllers.ListBooksByGenre)

			// Order management (admin)
			admin.GET("/orders", controllers.AdminListOrders)
			admin.GET("/orders/returns", controllers.AdminListReturnRequests)
			admin.GET("/orders/:id", controllers.AdminGetOrderDetails)
			admin.PUT("/orders/:id/status", controllers.AdminUpdateOrderStatus)

			// Return and refund management
			admin.POST("/orders/:id/return/approve", controllers.ApproveOrderReturn)
			admin.POST("/orders/:id/return/reject", controllers.RejectOrderReturn)
			admin.GET("/orders/return-items", controllers.AdminListReturnItems)
			admin.POST("/orders/:id/items/:item_id/review", controllers.AdminReviewReturnItem)

			// Coupon management
			admin.POST("/coupons", controllers.CreateCoupon)
			admin.GET("/coupons", controllers.GetCoupons)
			admin.PUT("/coupons/:id", controllers.UpdateCoupon)
			admin.DELETE("/coupons/:id", controllers.DeleteCoupon)

			// Product Offer routes
			adminOffers := admin.Group("/offers")
			adminOffers.POST("/products", controllers.CreateProductOffer)
			adminOffers.GET("/products", controllers.ListProductOffers)
			adminOffers.PUT("/products/:id", controllers.UpdateProductOffer)
			adminOffers.PATCH("/products/:id", controllers.UpdateProductOffer)
			adminOffers.DELETE("/products/:id", controllers.DeleteProductOffer)

			// Category Offer routes
			adminOffers.POST("/categories", controllers.CreateCategoryOffer)
			adminOffers.GET("/categories", controllers.ListCategoryOffers)
			adminOffers.PUT("/categories/:id", controllers.UpdateCategoryOffer)
			adminOffers.PATCH("/categories/:id", controllers.UpdateCategoryOffer)
			adminOffers.DELETE("/categories/:id", controllers.DeleteCategoryOffer)

			// Referral management (admin)
			admin.GET("/referrals", controllers.GetAllUserReferralCodes)
			admin.GET("/referrals/stats", controllers.GetReferralStatistics)
			admin.GET("/referrals/user/:user_id", controllers.GetUserReferralStats)
			admin.POST("/referrals/toggle", controllers.ToggleReferralCodeStatus)

			// Sales report endpoints (admin)
			admin.GET("/sales/report", controllers.GenerateSalesReport)
			admin.GET("/sales/report/pdf", controllers.DownloadSalesReportPDF)
			admin.GET("/sales/report/excel", controllers.DownloadSalesReportExcel)

			// Dashboard routes
			dashboard := admin.Group("/dashboard")
			{
				dashboard.GET("/stats", controllers.GetDashboardStats)
				dashboard.GET("/sales-chart", controllers.GetSalesChart)
				dashboard.GET("/top-products", controllers.GetTopSellingProducts)
				dashboard.GET("/top-categories", controllers.GetTopSellingCategories)
			}

			// Delivery charge management
			admin.GET("/delivery-charges", controllers.GetDeliveryCharges)
			admin.POST("/delivery-charges", controllers.AddDeliveryCharge)
			admin.PUT("/delivery-charges/:id", controllers.UpdateDeliveryCharge)
			admin.DELETE("/delivery-charges/:id", controllers.DeleteDeliveryCharge)
			admin.GET("/delivery-charges/pincode/:pincode", controllers.GetDeliveryChargeByPincode)
		}
	}

	utils.LogInfo("Admin routes registration completed")
}
