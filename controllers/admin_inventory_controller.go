package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
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
		q := strings.ToLower(blocked)
		if q == "true" {
			query = query.Where("blocked = ?", true)
		} else if q == "false" {
			query = query.Where("blocked = ?", false)
		}
	}

	// Sorting
	sort := c.DefaultQuery("sort", "id desc")
	query = query.Order(sort)

	// Pagination
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if page < 1 { page = 1 }
	if limit < 1 { limit = 20 }
	var total int64
	query.Model(&models.Book{}).Count(&total)
	query.Offset((page-1)*limit).Limit(limit).Find(&products)

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"total": total,
		"page": page,
		"limit": limit,
	})
}

// AdminBlockProduct blocks/unlists a product
func AdminBlockProduct(c *gin.Context) {
	productID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	var book models.Book
	if err := config.DB.First(&book, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	book.Blocked = true
	config.DB.Save(&book)
	c.JSON(http.StatusOK, gin.H{"message": "Product blocked/unlisted", "product": book})
}

// AdminUnblockProduct unblocks a product
func AdminUnblockProduct(c *gin.Context) {
	productID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product ID"})
		return
	}
	var book models.Book
	if err := config.DB.First(&book, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}
	book.Blocked = false
	config.DB.Save(&book)
	c.JSON(http.StatusOK, gin.H{"message": "Product unblocked/listed", "product": book})
}

// AdminBlockCategory blocks/unlists a category
func AdminBlockCategory(c *gin.Context) {
	catID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}
	var cat models.Category
	if err := config.DB.First(&cat, catID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}
	cat.Blocked = true
	config.DB.Save(&cat)
	c.JSON(http.StatusOK, gin.H{"message": "Category blocked/unlisted", "category": cat})
}

// AdminUnblockCategory unblocks a category
func AdminUnblockCategory(c *gin.Context) {
	catID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}
	var cat models.Category
	if err := config.DB.First(&cat, catID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Category not found"})
		return
	}
	cat.Blocked = false
	config.DB.Save(&cat)
	c.JSON(http.StatusOK, gin.H{"message": "Category unblocked/listed", "category": cat})
}
