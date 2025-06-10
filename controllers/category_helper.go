package controllers

import (
	"fmt"
	"log"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
)

// CreateDefaultCategory creates a default category if none exists
func CreateDefaultCategory() error {
	var count int64
	if err := config.DB.Model(&models.Category{}).Count(&count).Error; err != nil {
		return err
	}

	if count == 0 {
		log.Printf("No categories found, creating default category")
		defaultCategory := models.Category{
			Name:        "General",
			Description: "Default category for books",
		}
		return config.DB.Create(&defaultCategory).Error
	}

	return nil
}

// EnsureCategoryExists checks if a category exists by ID and creates it if it doesn't
func EnsureCategoryExists(categoryID uint) error {
	var category models.Category
	if err := config.DB.First(&category, categoryID).Error; err != nil {
		log.Printf("Category %d not found, creating it", categoryID)
		category = models.Category{
			Name:        fmt.Sprintf("Category %d", categoryID),
			Description: fmt.Sprintf("Description for category %d", categoryID),
		}
		return config.DB.Create(&category).Error
	}
	return nil
}

// EnsureGenreExists checks if a genre exists by ID and creates it if it doesn't
func EnsureGenreExists(genreID uint) error {
	var genre models.Genre
	if err := config.DB.First(&genre, genreID).Error; err != nil {
		log.Printf("Genre %d not found, creating it", genreID)
		genre = models.Genre{
			Name:        fmt.Sprintf("Genre %d", genreID),
			Description: fmt.Sprintf("Description for genre %d", genreID),
		}
		return config.DB.Create(&genre).Error
	}
	return nil
}
