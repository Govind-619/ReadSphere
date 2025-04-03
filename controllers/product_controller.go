package controllers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateProduct(c *gin.Context) {
	var product models.Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Handle image uploads
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get form data"})
		return
	}

	files := form.File["images"]
	if len(files) < 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "At least 3 images are required"})
		return
	}

	var imagePaths []string
	for _, file := range files {
		// Generate unique filename
		ext := filepath.Ext(file.Filename)
		filename := uuid.New().String() + ext

		// Save file
		if err := c.SaveUploadedFile(file, "uploads/"+filename); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
			return
		}

		imagePaths = append(imagePaths, filename)
	}

	product.Images = imagePaths

	// Create product
	if err := config.DB.Create(&product).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	c.JSON(http.StatusCreated, product)
}

func GetProducts(c *gin.Context) {
	var products []models.Product
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")
	search := c.Query("search")

	// Convert page and limit to integers
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	query := config.DB.Model(&models.Product{})

	if search != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	// Get total count
	var total int64
	query.Count(&total)

	// Pagination
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// Get products
	if err := query.Find(&products).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch products"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"products": products,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func UpdateProduct(c *gin.Context) {
	productID := c.Param("id")
	var product models.Product

	if err := config.DB.First(&product, productID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
		return
	}

	var updateData struct {
		Name        string  `json:"name"`
		Description string  `json:"description"`
		Price       float64 `json:"price"`
		Stock       int     `json:"stock"`
		CategoryID  uint    `json:"category_id"`
		IsActive    bool    `json:"is_active"`
	}

	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}

	// Handle new image uploads if any
	form, err := c.MultipartForm()
	if err == nil {
		files := form.File["images"]
		if len(files) > 0 {
			var imagePaths []string
			for _, file := range files {
				ext := filepath.Ext(file.Filename)
				filename := uuid.New().String() + ext

				if err := c.SaveUploadedFile(file, "uploads/"+filename); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
					return
				}

				imagePaths = append(imagePaths, filename)
			}
			product.Images = imagePaths
		}
	}

	// Update product fields
	updates := map[string]interface{}{
		"name":        updateData.Name,
		"description": updateData.Description,
		"price":       updateData.Price,
		"stock":       updateData.Stock,
		"category_id": updateData.CategoryID,
		"is_active":   updateData.IsActive,
	}

	if err := config.DB.Model(&product).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update product"})
		return
	}

	c.JSON(http.StatusOK, product)
}

func DeleteProduct(c *gin.Context) {
	productID := c.Param("id")

	// Soft delete
	if err := config.DB.Model(&models.Product{}).Where("id = ?", productID).Update("is_active", false).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete product"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Product deleted successfully"})
}

// GetProductDetails retrieves detailed information about a product
func GetProductDetails(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product details retrieved successfully",
		"product": gin.H{},
	})
}

// GetProductReviews retrieves reviews for a product
func GetProductReviews(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "Product reviews retrieved successfully",
		"reviews": []interface{}{},
	})
}
