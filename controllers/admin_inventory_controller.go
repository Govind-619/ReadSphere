package controllers

import (
	"strconv"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
)

// AdminListProductsWithStock shows products with stock, filter, sort, pagination
func AdminListProductsWithStock(c *gin.Context) {
	var products []models.Book
	query := config.DB

	// Filtering
	if name := c.Query("name"); name != "" {
		query = query.Where("name ILIKE ?", "%"+name+"%")
	}
	if category := c.Query("category"); category != "" {
		query = query.Where("category_id = ?", category)
	}
	if blocked := c.Query("blocked"); blocked != "" {
		switch q := strings.ToLower(blocked); q {
		case "true":
			query = query.Where("blocked = ?", true)
		case "false":
			query = query.Where("blocked = ?", false)
		}
	}

	// Sorting
	sort := c.DefaultQuery("sort", "id desc")
	query = query.Order(sort)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 20
	}
	var total int64
	query.Model(&models.Book{}).Count(&total)
	query.Offset((page - 1) * limit).Limit(limit).Find(&products)

	utils.Success(c, "Products retrieved successfully", gin.H{
		"products": products,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"limit":       limit,
			"total_pages": (total + int64(limit) - 1) / int64(limit),
		},
	})
}

// AdminBlockProduct blocks/unlists a product
func AdminBlockProduct(c *gin.Context) {
	productID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid product ID", err.Error())
		return
	}

	var book models.Book
	if err := config.DB.First(&book, productID).Error; err != nil {
		utils.NotFound(c, "Product not found")
		return
	}

	book.Blocked = true
	if err := config.DB.Save(&book).Error; err != nil {
		utils.InternalServerError(c, "Failed to block product", err.Error())
		return
	}

	utils.Success(c, "Product blocked successfully", gin.H{
		"product": book,
	})
}

// AdminUnblockProduct unblocks a product
func AdminUnblockProduct(c *gin.Context) {
	productID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid product ID", err.Error())
		return
	}

	var book models.Book
	if err := config.DB.First(&book, productID).Error; err != nil {
		utils.NotFound(c, "Product not found")
		return
	}

	book.Blocked = false
	if err := config.DB.Save(&book).Error; err != nil {
		utils.InternalServerError(c, "Failed to unblock product", err.Error())
		return
	}

	utils.Success(c, "Product unblocked successfully", gin.H{
		"product": book,
	})
}

// AdminBlockCategory blocks/unlists a category
func AdminBlockCategory(c *gin.Context) {
	catID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid category ID", err.Error())
		return
	}

	var cat models.Category
	if err := config.DB.First(&cat, catID).Error; err != nil {
		utils.NotFound(c, "Category not found")
		return
	}

	cat.Blocked = true
	if err := config.DB.Save(&cat).Error; err != nil {
		utils.InternalServerError(c, "Failed to block category", err.Error())
		return
	}

	utils.Success(c, "Category blocked successfully", gin.H{
		"category": cat,
	})
}

// AdminUnblockCategory unblocks a category
func AdminUnblockCategory(c *gin.Context) {
	catID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		utils.BadRequest(c, "Invalid category ID", err.Error())
		return
	}

	var cat models.Category
	if err := config.DB.First(&cat, catID).Error; err != nil {
		utils.NotFound(c, "Category not found")
		return
	}

	cat.Blocked = false
	if err := config.DB.Save(&cat).Error; err != nil {
		utils.InternalServerError(c, "Failed to unblock category", err.Error())
		return
	}

	utils.Success(c, "Category unblocked successfully", gin.H{
		"category": cat,
	})
}
