package controllers

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// RegisterRequest represents the registration request body
type RegisterRequest struct {
	Username        string `json:"username" binding:"required"`
	Email           string `json:"email" binding:"required,email"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
	Phone           string `json:"phone"`
}

// RegistrationData represents the registration data stored in session
type RegistrationData struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	OTP        string `json:"otp"`
	OTPExpires int64  `json:"otp_expires"`
	Username   string `json:"username"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Phone      string `json:"phone"`
}

// RegisterUser handles user registration
func RegisterUser(c *gin.Context) {
	var user models.User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": "Please check your input data and ensure all required fields are provided correctly.",
		})
		return
	}

	// Validate username
	if valid, msg := utils.ValidateUsername(user.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid username",
			"details": msg,
		})
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(user.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid email",
			"details": msg,
		})
		return
	}

	// Validate password
	if valid, msg := utils.ValidatePassword(user.Password); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid password",
			"details": msg,
		})
		return
	}

	// Validate first name if provided
	if user.FirstName != "" {
		if valid, msg := utils.ValidateName(user.FirstName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Validate last name if provided
	if user.LastName != "" {
		if valid, msg := utils.ValidateName(user.LastName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Validate phone if provided
	if user.Phone != "" {
		if valid, msg := utils.ValidatePhone(user.Phone); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Check for SQL injection in all fields
	if valid, msg := utils.ValidateSQLInjection(user.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(user.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(user.FirstName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(user.LastName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(user.Phone); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check for XSS in all fields
	if valid, msg := utils.ValidateXSS(user.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(user.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(user.FirstName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(user.LastName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(user.Phone); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := config.DB.Where("username = ?", user.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Username already exists",
			"details": "The username you've chosen is already taken. Please choose a different username.",
		})
		return
	}

	// Check if email already exists
	if err := config.DB.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Email already exists",
			"details": "An account with this email address already exists. Please use a different email or try logging in.",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process password",
			"details": "An error occurred while securing your password. Please try again later.",
		})
		return
	}
	user.Password = string(hashedPassword)

	// Create user
	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create user account",
			"details": "An error occurred while creating your account. Please try again later.",
		})
		return
	}

	// Generate OTP
	otp := generateOTP()
	otpExpiry := time.Now().Add(15 * time.Minute)

	// Store OTP
	if err := config.DB.Create(&models.UserOTP{
		UserID:    user.ID,
		Code:      otp,
		ExpiresAt: otpExpiry,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to generate verification code",
			"details": "An error occurred while generating your verification code. Please try again later.",
		})
		return
	}

	// Send OTP email
	if err := utils.SendOTP(user.Email, otp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send verification email",
			"details": "An error occurred while sending your verification email. Please try again later.",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "User registered successfully. Please check your email for verification code.",
		"user_id": user.ID,
	})
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginUser handles user login
func LoginUser(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid email or password"})
		return
	}

	// Sanitize input
	req.Email = utils.SanitizeString(req.Email)

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check for SQL injection
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check for XSS
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if user.IsBlocked {
		c.JSON(http.StatusForbidden, gin.H{"error": "Account is blocked"})
		return
	}

	// Update last login
	user.LastLoginAt = time.Now()
	config.DB.Save(&user)

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.Model.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
	})
}

// VerifyOTPRequest represents the OTP verification request body
type VerifyOTPRequest struct {
	OTP string `json:"otp" binding:"required"`
}

func VerifyOTP(c *gin.Context) {
	var req VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please provide OTP"})
		return
	}

	// Get registration data from session
	session := sessions.Default(c)
	email := session.Get("email")
	if email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please register first"})
		return
	}

	// Check if OTP has expired
	otpExpires := session.Get("otp_expires").(int64)
	if time.Now().Unix() > otpExpires {
		// Generate new OTP
		newOTP := generateOTP()
		log.Println("OTP expired, sending new OTP:", newOTP)

		// Update session with new OTP and expiration time
		session.Set("otp", newOTP)
		session.Set("otp_expires", time.Now().Add(time.Minute*1).Unix())

		if err := session.Save(); err != nil {
			log.Printf("Failed to save session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
			return
		}

		// Send new OTP via email
		if err := utils.SendOTP(email.(string), newOTP); err != nil {
			log.Printf("Failed to send OTP email: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "OTP has expired",
			"message":    "A new OTP has been sent to your email",
			"email":      email,
			"expires_in": "60 seconds",
		})
		return
	}

	// Verify OTP
	storedOTP := session.Get("otp").(string)
	if storedOTP != req.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	// Create user in database after successful verification
	user := models.User{
		Username:   session.Get("username").(string),
		Email:      email.(string),
		Password:   session.Get("password").(string),
		FirstName:  session.Get("first_name").(string),
		LastName:   session.Get("last_name").(string),
		Phone:      session.Get("phone").(string),
		IsVerified: true,
	}

	// Save user to database
	if err := config.DB.Create(&user).Error; err != nil {
		log.Printf("Database error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.Model.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		log.Printf("Token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Clear session after successful verification
	session.Clear()
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"message": "Email verified and registration completed successfully",
		"token":   tokenString,
	})
}

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please provide a valid email address"})
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if user exists
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Generate OTP
	otp := generateOTP()
	log.Println("Forgot Password OTP:", otp)

	// Store email and OTP in session
	session := sessions.Default(c)
	session.Set("reset_email", req.Email)
	session.Set("reset_otp", otp)
	session.Set("reset_otp_expires", time.Now().Add(time.Minute*1).Unix())

	if err := session.Save(); err != nil {
		log.Printf("Session save error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Send OTP via email
	if err := utils.SendOTP(req.Email, otp); err != nil {
		log.Printf("Email error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset OTP has been sent to your email",
		"email":   req.Email,
	})
}

// VerifyResetOTPRequest represents the reset password OTP verification request body
type VerifyResetOTPRequest struct {
	OTP string `json:"otp" binding:"required"`
}

func VerifyResetOTP(c *gin.Context) {
	var req VerifyResetOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please provide OTP"})
		return
	}

	// Get reset data from session
	session := sessions.Default(c)
	email := session.Get("reset_email")
	if email == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please request password reset first"})
		return
	}

	// Check if OTP has expired
	otpExpires := session.Get("reset_otp_expires").(int64)
	if time.Now().Unix() > otpExpires {
		// Generate new OTP
		newOTP := generateOTP()
		log.Println("Reset OTP expired, sending new OTP:", newOTP)

		// Update session with new OTP and expiration time
		session.Set("reset_otp", newOTP)
		session.Set("reset_otp_expires", time.Now().Add(time.Minute*1).Unix())

		if err := session.Save(); err != nil {
			log.Printf("Failed to save session: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
			return
		}

		// Send new OTP via email
		if err := utils.SendOTP(email.(string), newOTP); err != nil {
			log.Printf("Failed to send OTP email: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send verification email"})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "OTP has expired",
			"message":    "A new OTP has been sent to your email",
			"email":      email,
			"expires_in": "60 seconds",
		})
		return
	}

	// Verify OTP
	storedOTP := session.Get("reset_otp").(string)
	if storedOTP != req.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	// Generate a temporary token for password reset
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(15 * time.Minute).Unix(), // Token expires in 15 minutes
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate reset token"})
		return
	}

	// Store token in session before clearing
	session.Set("reset_token", tokenString)
	if err := session.Save(); err != nil {
		log.Printf("Failed to save session: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "OTP verified successfully. Please reset your password.",
		"token":   tokenString,
	})
}

// ResetPasswordRequest represents the reset password request body
type ResetPasswordRequest struct {
	NewPassword     string `json:"new_password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

func ResetPassword(c *gin.Context) {
	var req ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please provide new password and confirm password"})
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Validate confirm password
	if valid, msg := utils.ValidateConfirmPassword(req.NewPassword, req.ConfirmPassword); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Get token from session
	session := sessions.Default(c)
	tokenString := session.Get("reset_token")
	if tokenString == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Please verify your OTP first"})
		return
	}

	// Verify token
	token, err := jwt.Parse(tokenString.(string), func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired reset token"})
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		return
	}

	email := claims["email"].(string)

	// Get user from database
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.NewPassword)); err == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password cannot be the same as current password"})
		return
	}

	// Check password history (last 3 passwords)
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "This password has been used recently. Please choose a different password"})
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction"})
		return
	}

	// Update user's password
	if err := tx.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Add to password history
	passwordHistoryEntry := models.PasswordHistory{
		UserID:   user.ID,
		Password: string(hashedPassword),
	}
	if err := tx.Create(&passwordHistoryEntry).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password history"})
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction"})
		return
	}

	// Clear session
	session.Clear()
	session.Save()

	c.JSON(http.StatusOK, gin.H{
		"message": "Password reset successfully",
		"redirect": gin.H{
			"url":     "/login",
			"message": "Please login with your new password",
		},
	})
}

func AddReview(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Review added successfully"})
}

func generateOTP() string {
	rand.Seed(time.Now().UnixNano())
	otp := rand.Intn(900000) + 100000 // 6-digit OTP
	return fmt.Sprintf("%06d", otp)
}
