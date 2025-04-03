package config

import (
	"log"

	"github.com/Govind-619/ReadSphere/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// ConnectDatabase initializes the database connection and performs migrations
func ConnectDatabase() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	dsn := "host=" + config.DBHost + " user=" + config.DBUser + " password=" + config.DBPassword + " dbname=" + config.DBName + " port=" + config.DBPort + " sslmode=disable"

	var err2 error
	DB, err2 = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err2 != nil {
		log.Fatal("Failed to connect to database:", err2)
	}

	// Auto migrate the schema
	err = DB.AutoMigrate(
		&models.User{},
		&models.Product{},
		&models.Category{},
		&models.PasswordHistory{},
		// Future models to be added:
		// &models.Cart{},
		// &models.Wishlist{},
		// &models.Order{},
		// &models.OrderItem{},
	)
	if err != nil {
		log.Fatal("Failed to migrate database:", err)
	}
}
