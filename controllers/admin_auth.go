package controllers

import (
	"os"
	"strings"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// AdminLoginRequest represents the admin login request
type AdminLoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// AdminLogin handles admin authentication
func AdminLogin(c *gin.Context) {
	utils.LogInfo("AdminLogin called")
	var req AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Invalid login request: %v", err)
		utils.BadRequest(c, "Invalid input", err.Error())
		return
	}
	utils.LogDebug("Processing login request for email: %s", req.Email)

	var admin models.Admin
	if err := config.DB.Where("email = ?", req.Email).First(&admin).Error; err != nil {
		utils.LogError("Admin not found for email: %s: %v", req.Email, err)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}
	utils.LogDebug("Found admin record for email: %s", req.Email)

	if !admin.IsActive {
		utils.LogError("Inactive admin account attempted login: %s", admin.Email)
		utils.Forbidden(c, "Admin account is inactive")
		return
	}
	utils.LogDebug("Admin account is active: %s", admin.Email)

	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(req.Password)); err != nil {
		utils.LogError("Invalid password for admin: %s", admin.Email)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}
	utils.LogDebug("Password verified for admin: %s", admin.Email)

	// Update last login
	admin.LastLogin = time.Now()
	if err := config.DB.Save(&admin).Error; err != nil {
		utils.LogError("Failed to update last login for admin: %s: %v", admin.Email, err)
	} else {
		utils.LogDebug("Updated last login for admin: %s", admin.Email)
	}

	// Generate JWT token with simpler claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"admin_id": admin.ID,
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})
	utils.LogDebug("Generated JWT token for admin: %s", admin.Email)

	// Get JWT secret from environment
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		utils.LogError("JWT secret not configured")
		utils.InternalServerError(c, "JWT secret not configured", nil)
		return
	}
	utils.LogDebug("JWT secret length: %d", len(jwtSecret))

	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		utils.LogError("Failed to sign JWT token for admin: %s: %v", admin.Email, err)
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}
	utils.LogDebug("Successfully signed JWT token for admin: %s", admin.Email)

	utils.LogInfo("Admin login successful: %s", admin.Email)
	utils.Success(c, "Login successful", gin.H{
		"token": tokenString,
		"admin": gin.H{
			"id":        admin.ID,
			"email":     admin.Email,
			"firstName": admin.FirstName,
			"lastName":  admin.LastName,
		},
	})
}

// AdminLogout handles admin logout
func AdminLogout(c *gin.Context) {
	utils.LogInfo("AdminLogout called")

	// Get the token from the Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		utils.LogError("Missing Authorization header on logout")
		utils.Success(c, "Logged out successfully", nil)
		return
	}
	tokenString := authHeader
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenString = strings.TrimPrefix(authHeader, "Bearer ")
	}

	// Parse the token to get expiry
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil {
		utils.LogError("Failed to parse token on logout: %v", err)
		utils.Success(c, "Logged out successfully", nil)
		return
	}

	var expiresAt time.Time
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if exp, ok := claims["exp"].(float64); ok {
			expiresAt = time.Unix(int64(exp), 0)
		} else {
			expiresAt = time.Now().Add(24 * time.Hour) // fallback
		}
	} else {
		expiresAt = time.Now().Add(24 * time.Hour) // fallback
	}

	// Blacklist the token
	blacklisted := models.BlacklistedToken{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	}
	if err := config.DB.Create(&blacklisted).Error; err != nil {
		utils.LogError("Failed to blacklist token on logout: %v", err)
	}

	utils.LogDebug("Client-side logout processed and token blacklisted")
	utils.Success(c, "Logged out successfully", nil)
}

// CreateSampleAdmin creates a sample admin user
func CreateSampleAdmin() error {
	utils.LogInfo("CreateSampleAdmin called")
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(os.Getenv("ADMIN_PASSWORD")), bcrypt.DefaultCost)
	if err != nil {
		utils.LogError("Failed to hash admin password: %v", err)
		return err
	}
	utils.LogDebug("Successfully hashed admin password")

	admin := models.Admin{
		Email:     os.Getenv("ADMIN_EMAIL"),
		Password:  string(hashedPassword),
		FirstName: os.Getenv("ADMIN_FIRST_NAME"),
		LastName:  os.Getenv("ADMIN_LAST_NAME"),
		IsActive:  true,
	}
	utils.LogDebug("Created admin model for email: %s", admin.Email)

	err = config.DB.FirstOrCreate(&admin, models.Admin{Email: admin.Email}).Error
	if err != nil {
		utils.LogError("Failed to create sample admin: %v", err)
		return err
	}
	utils.LogInfo("Successfully created/updated sample admin: %s", admin.Email)
	return nil
}
