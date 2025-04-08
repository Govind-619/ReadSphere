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

	// Remove unique constraint on ISBN field if it exists
	removeISBNUniqueConstraint()

	// Auto migrate the schema
	err = DB.AutoMigrate(
		&models.User{},
		&models.Book{},
		&models.Category{},
		&models.Genre{},
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

// removeISBNUniqueConstraint removes the unique constraint on the ISBN field
func removeISBNUniqueConstraint() {
	// Check if the constraint exists
	var constraintExists bool
	err := DB.Raw(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.table_constraints 
			WHERE constraint_name = 'books_isbn_key'
		)
	`).Scan(&constraintExists).Error
	if err != nil {
		log.Printf("Failed to check ISBN constraint: %v", err)
		return
	}

	// If the constraint exists, drop it
	if constraintExists {
		log.Printf("Removing unique constraint on ISBN field")
		err = DB.Exec(`ALTER TABLE books DROP CONSTRAINT books_isbn_key`).Error
		if err != nil {
			log.Printf("Failed to remove ISBN constraint: %v", err)
		}
	}
}
