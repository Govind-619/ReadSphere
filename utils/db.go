package utils

import (
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// CreateUser creates a new user
func CreateUser(user *models.User) error {
	return config.DB.Create(user).Error
}

// GetUserByID retrieves a user by ID
func GetUserByID(id uint) (*models.User, error) {
	var user models.User
	err := config.DB.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := config.DB.Where("email = ?", email).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// UpdateUser updates a user
func UpdateUser(user *models.User) error {
	return config.DB.Save(user).Error
}

// DeleteUser deletes a user
func DeleteUser(id uint) error {
	return config.DB.Delete(&models.User{}, id).Error
}

// CreateAdmin creates a new admin
func CreateAdmin(admin *models.Admin) error {
	return config.DB.Create(admin).Error
}

// GetAdminByEmail retrieves an admin by email
func GetAdminByEmail(email string) (*models.Admin, error) {
	var admin models.Admin
	err := config.DB.Where("email = ?", email).First(&admin).Error
	if err != nil {
		return nil, err
	}
	return &admin, nil
}

// CreateCategory creates a new category
func CreateCategory(category *models.Category) error {
	return config.DB.Create(category).Error
}

// GetCategoryByID retrieves a category by ID
func GetCategoryByID(id uint) (*models.Category, error) {
	var category models.Category
	err := config.DB.First(&category, id).Error
	if err != nil {
		return nil, err
	}
	return &category, nil
}

// UpdateCategory updates a category
func UpdateCategory(category *models.Category) error {
	return config.DB.Save(category).Error
}

// DeleteCategory deletes a category
func DeleteCategory(id uint) error {
	return config.DB.Delete(&models.Category{}, id).Error
}

// CreateProduct creates a new product
func CreateProduct(product *models.Product) error {
	return config.DB.Create(product).Error
}

// GetProductByID retrieves a product by ID
func GetProductByID(id uint) (*models.Product, error) {
	var product models.Product
	err := config.DB.Preload("Category").Preload("Reviews").First(&product, id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// UpdateProduct updates a product
func UpdateProduct(product *models.Product) error {
	return config.DB.Save(product).Error
}

// DeleteProduct deletes a product
func DeleteProduct(id uint) error {
	return config.DB.Delete(&models.Product{}, id).Error
}

// CreateReview creates a new review
func CreateReview(review *models.Review) error {
	return config.DB.Create(review).Error
}

// GetReviewByID retrieves a review by ID
func GetReviewByID(id uint) (*models.Review, error) {
	var review models.Review
	err := config.DB.Preload("User").Preload("Product").First(&review, id).Error
	if err != nil {
		return nil, err
	}
	return &review, nil
}

// UpdateReview updates a review
func UpdateReview(review *models.Review) error {
	return config.DB.Save(review).Error
}

// DeleteReview deletes a review
func DeleteReview(id uint) error {
	return config.DB.Delete(&models.Review{}, id).Error
}

// GetProductsByCategory retrieves products by category ID with pagination
func GetProductsByCategory(categoryID uint, page, limit int) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	// Get total count
	err := config.DB.Model(&models.Product{}).Where("category_id = ?", categoryID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated products
	offset := (page - 1) * limit
	err = config.DB.Preload("Category").
		Where("category_id = ?", categoryID).
		Offset(offset).
		Limit(limit).
		Find(&products).Error
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// SearchProducts searches products by name or description
func SearchProducts(query string, page, limit int) ([]models.Product, int64, error) {
	var products []models.Product
	var total int64

	// Get total count
	err := config.DB.Model(&models.Product{}).
		Where("name ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%").
		Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated products
	offset := (page - 1) * limit
	err = config.DB.Preload("Category").
		Where("name ILIKE ? OR description ILIKE ?", "%"+query+"%", "%"+query+"%").
		Offset(offset).
		Limit(limit).
		Find(&products).Error
	if err != nil {
		return nil, 0, err
	}

	return products, total, nil
}

// GetUserReviews retrieves reviews by user ID with pagination
func GetUserReviews(userID uint, page, limit int) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	// Get total count
	err := config.DB.Model(&models.Review{}).Where("user_id = ?", userID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated reviews
	offset := (page - 1) * limit
	err = config.DB.Preload("Product").
		Where("user_id = ?", userID).
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// GetProductReviews retrieves reviews by product ID with pagination
func GetProductReviews(productID uint, page, limit int) ([]models.Review, int64, error) {
	var reviews []models.Review
	var total int64

	// Get total count
	err := config.DB.Model(&models.Review{}).Where("product_id = ?", productID).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	// Get paginated reviews
	offset := (page - 1) * limit
	err = config.DB.Preload("User").
		Where("product_id = ?", productID).
		Offset(offset).
		Limit(limit).
		Find(&reviews).Error
	if err != nil {
		return nil, 0, err
	}

	return reviews, total, nil
}

// UpdateUserOTP updates a user's OTP and OTP expiry
func UpdateUserOTP(userID uint, otp string) error {
	expiry := time.Now().Add(10 * time.Minute)
	return config.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"otp":        otp,
			"otp_expiry": expiry,
		}).Error
}

// BlockUser blocks a user
func BlockUser(userID uint) error {
	return config.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Update("is_blocked", true).Error
}

// UnblockUser unblocks a user
func UnblockUser(userID uint) error {
	return config.DB.Model(&models.User{}).
		Where("id = ?", userID).
		Update("is_blocked", false).Error
}
