package middleware

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.LogInfo("AuthMiddleware called")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.LogError("Missing Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		// Extract token from Bearer header
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(os.Getenv("JWT_SECRET")), nil
		})

		if err != nil {
			utils.LogError("Invalid token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		if !token.Valid {
			utils.LogError("Token validation failed")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			utils.LogError("Invalid token claims")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Get user from database
		userID := uint(claims["user_id"].(float64))
		utils.LogDebug("Authenticating user ID: %d", userID)

		var user models.User
		if err := config.DB.First(&user, userID).Error; err != nil {
			utils.LogError("User not found: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			c.Abort()
			return
		}

		if user.IsBlocked {
			utils.LogError("Blocked user attempted access: %d", userID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is blocked"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		utils.LogInfo("User %d authenticated successfully", userID)
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.LogInfo("AdminMiddleware called")

		user, exists := c.Get("user")
		if !exists {
			utils.LogError("User not found in context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found in context"})
			c.Abort()
			return
		}

		userModel, ok := user.(models.User)
		if !ok {
			utils.LogError("Invalid user type in context")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user type"})
			c.Abort()
			return
		}

		if !userModel.IsAdmin {
			utils.LogError("Non-admin user attempted admin access: %d", userModel.ID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}

		utils.LogInfo("Admin access granted for user %d", userModel.ID)
		c.Next()
	}
}

func AdminAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.LogInfo("AdminAuthMiddleware called")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.LogError("Missing Authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
			c.Abort()
			return
		}

		// Extract token from Bearer header
		tokenString := strings.Replace(authHeader, "Bearer ", "", 1)
		if tokenString == authHeader {
			utils.LogError("Invalid Bearer token format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		// Get JWT secret from environment
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			utils.LogError("JWT secret not configured")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "JWT secret not configured"})
			c.Abort()
			return
		}

		// Parse and validate token
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(jwtSecret), nil
		})

		if err != nil {
			utils.LogError("Invalid admin token: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		if !token.Valid {
			utils.LogError("Admin token validation failed")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		// Extract claims
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			utils.LogError("Invalid admin token claims")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		// Get admin ID from claims
		adminID, ok := claims["admin_id"].(float64)
		if !ok {
			utils.LogError("Admin ID not found in token claims")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Please login for access"})
			c.Abort()
			return
		}

		utils.LogDebug("Authenticating admin ID: %d", uint(adminID))

		// Get admin from database
		var admin models.Admin
		if err := config.DB.First(&admin, uint(adminID)).Error; err != nil {
			utils.LogError("Admin not found: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Admin not found"})
			c.Abort()
			return
		}

		if !admin.IsActive {
			utils.LogError("Inactive admin attempted access: %d", admin.ID)
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin account is inactive"})
			c.Abort()
			return
		}

		// Set admin in context
		c.Set("admin", admin)
		utils.LogInfo("Admin %d authenticated successfully", admin.ID)
		c.Next()
	}
}
