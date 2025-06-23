package config

import (
	"fmt"
	"log"
	"os"
	"strings"

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

// MigrateCategoryNames standardizes category names and removes duplicates
func MigrateCategoryNames(db *gorm.DB) error {
	// Start transaction
	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Drop any existing indexes on the name column
	if err := tx.Exec(`DROP INDEX IF EXISTS idx_categories_name_lower`).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Exec(`DROP INDEX IF EXISTS categories_name_key`).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Exec(`DROP INDEX IF EXISTS idx_categories_name`).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Get all categories
	var categories []models.Category
	if err := tx.Find(&categories).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Process categories and handle duplicates
	seen := make(map[string]*models.Category)
	for i := range categories {
		cat := &categories[i]
		normalizedName := strings.TrimSpace(cat.Name)
		lowerName := strings.ToLower(normalizedName)

		if existing, exists := seen[lowerName]; exists {
			// Update books to use the first category we saw
			if err := tx.Model(&models.Book{}).Where("category_id = ?", cat.ID).Update("category_id", existing.ID).Error; err != nil {
				tx.Rollback()
				return err
			}
			// Mark for deletion by setting DeletedAt
			if err := tx.Delete(cat).Error; err != nil {
				tx.Rollback()
				return err
			}
		} else {
			// Update the name to be standardized
			cat.Name = normalizedName
			if err := tx.Save(cat).Error; err != nil {
				tx.Rollback()
				return err
			}
			seen[lowerName] = cat
		}
	}

	// Create the case-insensitive unique index
	if err := tx.Exec(`CREATE UNIQUE INDEX idx_categories_name_lower ON categories (LOWER(name)) WHERE deleted_at IS NULL`).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// CleanupInvalidReferrals removes any referral records that reference non-existent coupons
func CleanupInvalidReferrals() error {
	// Check if referrals table exists
	if !DB.Migrator().HasTable("referrals") {
		return nil
	}

	// Delete referrals with invalid coupon_id
	result := DB.Exec(`
		DELETE FROM referrals 
		WHERE coupon_id NOT IN (SELECT id FROM coupons)
	`)
	if result.Error != nil {
		log.Printf("Failed to cleanup invalid referrals: %v", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		log.Printf("Cleaned up %d invalid referral records", result.RowsAffected)
	}
	return nil
}

// MigrateDB migrates the database
func MigrateDB() error {
	// Clean up invalid referrals before migration
	if err := CleanupInvalidReferrals(); err != nil {
		log.Printf("Warning: Failed to cleanup invalid referrals: %v", err)
		// Continue with migration even if cleanup fails
	}

	// AutoMigrate will create tables, missing foreign keys, constraints, columns and indexes
	if err := DB.AutoMigrate(
		&models.User{},
		&models.Admin{},
		&models.Book{},
		&models.Category{},
		&models.Cart{},
		&models.Address{},
		&models.Order{},
		&models.OrderItem{},
		&models.Wishlist{},
		&models.Coupon{},           // Migrate Coupon first
		&models.UserReferralCode{}, // New referral system
		&models.ReferralUsage{},    // New referral usage tracking
		&models.Referral{},         // Legacy - can be removed later
		&models.ReferralSignup{},
		&models.ProductOffer{},
		&models.CategoryOffer{},
		&models.Wallet{},
		&models.WalletTransaction{},
		&models.WalletTopupOrder{},
		&models.BlacklistedToken{},
	); err != nil {
		log.Printf("Failed to migrate database: %v", err)
		return err
	}

	return nil
}

// InitDB initializes the database connection
func InitDB() {
	config, err := LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
		config.DBHost, config.DBUser, config.DBPassword, config.DBName, config.DBPort)

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Run category migration first
	if err := MigrateCategoryNames(DB); err != nil {
		log.Fatal("Failed to migrate category names:", err)
	}

	// Then run the regular migrations
	if err := MigrateDB(); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

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

	// Update GoogleID column to be nullable
	err = DB.Exec(`
		ALTER TABLE users 
		ALTER COLUMN google_id DROP NOT NULL,
		ALTER COLUMN google_id SET DEFAULT NULL
	`).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to update google_id column: %v", err))
	}

	// Drop existing index if it exists
	err = DB.Exec(`DROP INDEX IF EXISTS idx_coupons_code_lower`).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to drop existing index: %v", err))
	}

	// Update existing coupon codes to uppercase
	err = DB.Exec(`UPDATE coupons SET code = UPPER(code) WHERE code != UPPER(code)`).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to update existing coupon codes: %v", err))
	}

	// Create case-insensitive unique index on coupon code
	err = DB.Exec(`
		CREATE UNIQUE INDEX idx_coupons_code_lower 
		ON coupons (LOWER(code))
	`).Error
	if err != nil {
		panic(fmt.Sprintf("Failed to create case-insensitive index: %v", err))
	}
}
