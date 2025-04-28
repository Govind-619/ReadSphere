package config

import (
	"fmt"
	"os"

	"github.com/Govind-619/ReadSphere/models"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

// Config holds all configuration for the application
type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	JWTSecret  string
	Port       string
	Env        string
}

// LoadConfig loads configuration from environment variables
func LoadConfig() (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, fmt.Errorf("error loading .env file: %v", err)
	}

	config := &Config{
		DBHost:     os.Getenv("DB_HOST"),
		DBPort:     os.Getenv("DB_PORT"),
		DBUser:     os.Getenv("DB_USER"),
		DBPassword: os.Getenv("DB_PASSWORD"),
		DBName:     os.Getenv("DB_NAME"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
		Port:       os.Getenv("PORT"),
		Env:        os.Getenv("ENV"),
	}

	return config, nil
}

// InitDB initializes the database connection
func InitDB() {
	config, err := LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	DB = db

	// First, check if the username column exists
	var columnExists bool
	err = DB.Raw(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'users' 
			AND column_name = 'username'
		)
	`).Scan(&columnExists).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to check column existence: %v", err))
	}

	if !columnExists {
		// Add username column as nullable first
		err = DB.Exec(`ALTER TABLE users ADD COLUMN username text`).Error
		if err != nil {
			panic(fmt.Sprintf("Failed to add username column: %v", err))
		}

		// Update existing users with default usernames
		err = DB.Exec(`
			UPDATE users 
			SET username = 'user_' || id::text 
			WHERE username IS NULL
		`).Error
		if err != nil {
			panic(fmt.Sprintf("Failed to update existing users: %v", err))
		}

		// Make username column NOT NULL
		err = DB.Exec(`ALTER TABLE users ALTER COLUMN username SET NOT NULL`).Error
		if err != nil {
			panic(fmt.Sprintf("Failed to make username NOT NULL: %v", err))
		}
	}

	// Auto-migrate the schema for other changes
	err = DB.AutoMigrate(
		&models.User{},
		&models.Admin{},
		&models.Category{},
		&models.Genre{},
		&models.Book{},
		&models.Review{},
		&models.PasswordHistory{},
		&models.UserOTP{},
		&models.Cart{},
		&models.Wishlist{},
		&models.Order{},
		&models.OrderItem{},
		&models.Address{},
		&models.BookImage{},
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to migrate database: %v", err))
	}

	// Update GoogleID column to be nullable
	err = DB.Exec(`
		ALTER TABLE users 
		ALTER COLUMN google_id DROP NOT NULL,
		ALTER COLUMN google_id SET DEFAULT NULL
	`).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to update google_id column: %v", err))
	}
}
