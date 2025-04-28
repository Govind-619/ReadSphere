package controllers

import (
	"crypto/rand"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
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
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": "Please check your input data and ensure all required fields are provided correctly.",
		})
		return
	}

	// Validate username
	if valid, msg := utils.ValidateUsername(req.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid username",
			"details": msg,
		})
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid email",
			"details": msg,
		})
		return
	}

	// Validate password
	if valid, msg := utils.ValidatePassword(req.Password); !valid {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid password",
			"details": msg,
		})
		return
	}

	// Confirm password match
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Passwords do not match",
			"details": "Password and confirm password must be the same.",
		})
		return
	}

	// Validate first name if provided
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Validate last name if provided
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Validate phone if provided
	if req.Phone != "" {
		if valid, msg := utils.ValidatePhone(req.Phone); !valid {
			c.JSON(http.StatusBadRequest, gin.H{"error": msg})
			return
		}
	}

	// Check for SQL injection in all fields
	if valid, msg := utils.ValidateSQLInjection(req.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.FirstName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.LastName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Phone); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check for XSS in all fields
	if valid, msg := utils.ValidateXSS(req.Username); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(req.FirstName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(req.LastName); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if valid, msg := utils.ValidateXSS(req.Phone); !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := config.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Username already exists",
			"details": "The username you've chosen is already taken. Please choose a different username.",
		})
		return
	}

	// Check if email already exists
	if err := config.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"error":   "Email already exists",
			"details": "An account with this email address already exists. Please use a different email or try logging in.",
		})
		return
	}

	//Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to process password",
			"details": "An error occurred while securing your password. Please try again later.",
		})
		return
	}

	// Generate OTP and expiry
	otp := generateOTP()
	log.Println("Registration OTP:", otp)
	otpExpiry := time.Now().Add(1 * time.Minute).Unix()
	regExpiry := time.Now().Add(15 * time.Minute).Unix()

	// Create JWT with registration info (NO OTP in claims)
	claims := jwt.MapClaims{
		"username":    req.Username,
		"email":       req.Email,
		"password":    string(hashedPassword),
		"first_name":  req.FirstName,
		"last_name":   req.LastName,
		"phone":       req.Phone,
		"exp":         regExpiry,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate registration token"})
		return
	}

	// Store OTP and expiry in session
	session := sessions.Default(c)
	session.Set("registration_otp", otp)
	session.Set("registration_otp_expires", otpExpiry)
	session.Set("registration_email", req.Email)
	if err := session.Save(); err != nil {
		log.Printf("Session save error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
		return
	}

	// Send OTP email
	log.Printf("[OTP RESEND] Registration OTP sent to %s: %s", req.Email, otp)
	if err := utils.SendOTP(req.Email, otp); err != nil {
		log.Printf("[OTP RESEND ERROR] Failed to send OTP to %s: %v", req.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to send verification email",
			"details": "An error occurred while sending your verification email. Please try again later.",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":            "OTP sent to your email. Please verify to complete registration.",
		"registration_token": tokenString,
		"expires_in":         900,
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
	var req struct {
		OTP               string `json:"otp" binding:"required"`
		RegistrationToken string `json:"registration_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Please provide OTP"})
		return
	}

	// Try to get token from Authorization header first
	regToken := req.RegistrationToken
	if authHeader := c.GetHeader("Authorization"); authHeader != "" {
		if strings.HasPrefix(authHeader, "Bearer ") {
			regToken = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}
	if regToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Registration token not provided"})
		return
	}

	// Parse the registration JWT
	token, err := jwt.Parse(regToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired registration token"})
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
		return
	}

	email := claims["email"].(string)

	// Use session to get OTP and expiry
	session := sessions.Default(c)
	sessEmail := session.Get("registration_email")
	if sessEmail == nil || sessEmail.(string) != email {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session expired or email mismatch. Please register again."})
		return
	}
	storedOTP := session.Get("registration_otp")
	otpExpires := session.Get("registration_otp_expires")
	if storedOTP == nil || otpExpires == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OTP session expired. Please register again."})
		return
	}
	if time.Now().Unix() > otpExpires.(int64) {
		// Check if registration token is still valid
		regExpires := int64(claims["exp"].(float64))
		if time.Now().Unix() > regExpires {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Registration expired. Please register again."})
			return
		}
		// Resend new OTP
		newOTP := generateOTP()
		log.Printf("[OTP RESEND] Registration OTP sent to %s: %s", email, newOTP)
		session.Set("registration_otp", newOTP)
		session.Set("registration_otp_expires", time.Now().Add(1 * time.Minute).Unix())
		if err := session.Save(); err != nil {
			log.Printf("Session save error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save session"})
			return
		}
		if err := utils.SendOTP(email, newOTP); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resend OTP"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{
			"error":        "OTP expired. A new OTP has been sent to your email.",
			"expires_in":   regExpires - time.Now().Unix(),
		})
		return
	}
	if storedOTP.(string) != req.OTP {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid OTP"})
		return
	}

	// Check if user already exists
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "User already exists. Please login."})
		return
	}

	// Create user
	user = models.User{
		Username:   claims["username"].(string),
		Email:      email,
		Password:   claims["password"].(string),
		FirstName:  claims["first_name"].(string),
		LastName:   claims["last_name"].(string),
		Phone:      claims["phone"].(string),
		IsVerified: true,
	}
	if err := config.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user account"})
		return
	}

	// Generate JWT token for login
	loginToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	tokenString, err := loginToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		log.Printf("Token generation error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

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
	// Use crypto/rand for secure random number generation
	b := make([]byte, 6)
	for i := 0; i < 6; i++ {
		num := 0
		for {
			r, err := rand.Int(rand.Reader, big.NewInt(10))
			if err == nil {
				num = int(r.Int64())
				break
			}
		}
		b[i] = byte('0' + num)
	}
	return string(b)
}
