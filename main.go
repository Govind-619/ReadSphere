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
	// Register types for session serialization
	gob.Register(controllers.RegistrationData{})

	// Load environment variables
	_, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	// Initialize database
	config.InitDB()

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

	// Start server
	if err := router.Run(":8080"); err != nil {
		log.Fatal("Error starting server:", err)
	}
}
