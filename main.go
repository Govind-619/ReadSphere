package main

import (
	"encoding/gob"
	"log"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/controllers"
	"github.com/Govind-619/ReadSphere/routes"
	"github.com/Govind-619/ReadSphere/utils"
)

func main() {
	// Initialize logger
	if err := utils.InitLogger(); err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	// Register types for session serialization
	gob.Register(controllers.RegistrationData{})

	// Load environment variables
	_, err := config.LoadConfig()
	if err != nil {
		utils.LogError("Error loading config: %v", err)
		log.Fatal("Error loading config:", err)
	}

	// Initialize database
	config.InitDB()

	// Create sample admin
	if err := controllers.CreateSampleAdmin(); err != nil {
		utils.LogError("Failed to create sample admin: %v", err)
		log.Fatal("Failed to create sample admin:", err)
	}

	// Create default category if none exists
	if err := controllers.CreateDefaultCategory(); err != nil {
		utils.LogError("Failed to create default category: %v", err)
		log.Fatal("Failed to create default category:", err)
	}

	// Initialize Google OAuth
	config.InitGoogleOAuth()

	// Set up router
	router := routes.SetupRouter()

	// Add middleware
	router.Use(utils.LoggerMiddleware())
	router.Use(utils.CORSMiddleware())
	router.Use(utils.RecoveryMiddleware())
	router.Use(utils.RequestIDMiddleware())
	router.Use(utils.SecurityHeadersMiddleware())

	utils.LogInfo("Server starting on port 8080")
	// Start server
	if err := router.Run(":8080"); err != nil {
		utils.LogError("Error starting server: %v", err)
		log.Fatal("Error starting server:", err)
	}
}
