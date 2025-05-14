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
	ReferralCode    string `json:"referral_code"`
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
		utils.BadRequest(c, "Invalid request format", "Please check your input data and ensure all required fields are provided correctly.")
		return
	}

	// Validate username
	if valid, msg := utils.ValidateUsername(req.Username); !valid {
		utils.BadRequest(c, "Invalid username", msg)
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Validate password
	if valid, msg := utils.ValidatePassword(req.Password); !valid {
		utils.BadRequest(c, "Invalid password", msg)
		return
	}

	// Confirm password match
	if req.Password != req.ConfirmPassword {
		utils.BadRequest(c, "Passwords do not match", "Password and confirm password must be the same.")
		return
	}

	// Validate first name if provided
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			utils.BadRequest(c, "Invalid first name", msg)
			return
		}
	}

	// Validate last name if provided
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			utils.BadRequest(c, "Invalid last name", msg)
			return
		}
	}

	// Validate phone if provided
	if req.Phone != "" {
		if valid, msg := utils.ValidatePhone(req.Phone); !valid {
			utils.BadRequest(c, "Invalid phone", msg)
			return
		}
	}

	// Check for SQL injection in all fields
	if valid, msg := utils.ValidateSQLInjection(req.Username); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.FirstName); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.LastName); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Phone); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check for XSS in all fields
	if valid, msg := utils.ValidateXSS(req.Username); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.FirstName); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.LastName); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.Phone); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := config.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		utils.Conflict(c, "Username already exists", "The username you've chosen is already taken. Please choose a different username.")
		return
	}

	// Check if email already exists
	if err := config.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		utils.Conflict(c, "Email already exists", "An account with this email address already exists. Please use a different email or try logging in.")
		return
	}

	//Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalServerError(c, "Failed to process password", "An error occurred while securing your password. Please try again later.")
		return
	}

	// Generate OTP and expiry
	otp := generateOTP()
	log.Println("Registration OTP:", otp)
	otpExpiry := time.Now().Add(1 * time.Minute).Unix()
	regExpiry := time.Now().Add(15 * time.Minute).Unix()

	// Create JWT with registration info (NO OTP in claims)
	claims := jwt.MapClaims{
		"username":      req.Username,
		"email":         req.Email,
		"password":      string(hashedPassword),
		"first_name":    req.FirstName,
		"last_name":     req.LastName,
		"phone":         req.Phone,
		"referral_code": req.ReferralCode,
		"exp":           regExpiry,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		utils.InternalServerError(c, "Failed to generate registration token", err.Error())
		return
	}

	// Store OTP and expiry in session
	session := sessions.Default(c)
	session.Set("registration_otp", otp)
	session.Set("registration_otp_expires", otpExpiry)
	session.Set("registration_email", req.Email)
	if err := session.Save(); err != nil {
		log.Printf("Session save error: %v", err)
		utils.InternalServerError(c, "Failed to save session", err.Error())
		return
	}

	// Send OTP email
	log.Printf("[OTP RESEND] Registration OTP sent to %s: %s", req.Email, otp)
	if err := utils.SendOTP(req.Email, otp); err != nil {
		log.Printf("[OTP RESEND ERROR] Failed to send OTP to %s: %v", req.Email, err)
		utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending your verification email. Please try again later.")
		return
	}

	utils.Success(c, "OTP sent to your email. Please verify to complete registration.", gin.H{
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
		utils.BadRequest(c, "Invalid email or password", err.Error())
		return
	}

	// Sanitize input
	req.Email = utils.SanitizeString(req.Email)

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Check for SQL injection
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check for XSS
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	if user.IsBlocked {
		utils.Forbidden(c, "Account is blocked")
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
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}

	utils.Success(c, "Login successful", gin.H{
		"token": tokenString, "user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
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
		utils.BadRequest(c, "Invalid request format", "Please provide OTP")
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
		utils.BadRequest(c, "Registration token missing", "Registration token not provided")
		return
	}

	// Parse the registration JWT
	token, err := jwt.Parse(regToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		utils.Unauthorized(c, "Invalid or expired registration token")
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		utils.Unauthorized(c, "Invalid token claims")
		return
	}

	email := claims["email"].(string)

	// Use session to get OTP and expiry
	session := sessions.Default(c)
	sessEmail := session.Get("registration_email")
	if sessEmail == nil || sessEmail.(string) != email {
		utils.BadRequest(c, "Session expired", "Session expired or email mismatch. Please register again.")
		return
	}
	storedOTP := session.Get("registration_otp")
	otpExpires := session.Get("registration_otp_expires")
	if storedOTP == nil || otpExpires == nil {
		utils.BadRequest(c, "OTP session expired", "OTP session expired. Please register again.")
		return
	}
	if time.Now().Unix() > otpExpires.(int64) {
		// Check if registration token is still valid
		regExpires := int64(claims["exp"].(float64))
		if time.Now().Unix() > regExpires {
			utils.BadRequest(c, "Registration expired", "Registration expired. Please register again.")
			return
		}
		// Resend new OTP
		newOTP := generateOTP()
		log.Printf("[OTP RESEND] Registration OTP sent to %s: %s", email, newOTP)
		session.Set("registration_otp", newOTP)
		session.Set("registration_otp_expires", time.Now().Add(1*time.Minute).Unix())
		if err := session.Save(); err != nil {
			log.Printf("Session save error: %v", err)
			utils.InternalServerError(c, "Failed to save session", err.Error())
			return
		}
		if err := utils.SendOTP(email, newOTP); err != nil {
			utils.InternalServerError(c, "Failed to resend OTP", "Failed to send verification email")
			return
		}
		utils.BadRequest(c, "OTP expired", gin.H{
			"message":    "OTP expired. A new OTP has been sent to your email.",
			"expires_in": regExpires - time.Now().Unix(),
		})
		return
	}
	if storedOTP.(string) != req.OTP {
		utils.BadRequest(c, "Invalid OTP", "The OTP you entered is incorrect")
		return
	}

	// Check if user already exists
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err == nil {
		utils.Conflict(c, "User already exists", "An account with this email already exists. Please login.")
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
		utils.InternalServerError(c, "Failed to create user account", err.Error())
		return
	}

	// Process referral code if present in registration claims
	if referralCode, ok := claims["referral_code"].(string); ok && referralCode != "" {
		err := AcceptReferralCodeAtSignup(user.ID, referralCode)
		if err != nil {
			utils.BadRequest(c, "Invalid referral code", "Invalid or expired referral code")
			return
		}
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
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}

	utils.Success(c, "Email verified and registration completed successfully", gin.H{
		"token": tokenString,
		"user": gin.H{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
		},
	})
}

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", "Please provide a valid email address")
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Check if user exists
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.NotFound(c, "User not found, No account exists with this email address")
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
		utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
		return
	}

	// Send OTP via email
	if err := utils.SendOTP(req.Email, otp); err != nil {
		log.Printf("Email error: %v", err)
		utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending the verification email. Please try again later.")
		return
	}

	utils.Success(c, "Password reset OTP has been sent to your email", gin.H{
		"email":      req.Email,
		"expires_in": 60, // OTP expires in 60 seconds
	})
}

// VerifyResetOTPRequest represents the reset password OTP verification request body
type VerifyResetOTPRequest struct {
	OTP string `json:"otp" binding:"required"`
}

func VerifyResetOTP(c *gin.Context) {
	var req VerifyResetOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request format", "Please provide OTP")
		return
	}

	// Get reset data from session
	session := sessions.Default(c)
	email := session.Get("reset_email")
	if email == nil {
		utils.BadRequest(c, "Invalid request", "Please request password reset first")
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
			utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
			return
		}

		// Send new OTP via email
		if err := utils.SendOTP(email.(string), newOTP); err != nil {
			log.Printf("Failed to send OTP email: %v", err)
			utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending the verification email. Please try again later.")
			return
		}

		utils.BadRequest(c, "OTP expired", gin.H{
			"message":    "A new OTP has been sent to your email",
			"email":      email,
			"expires_in": 60,
		})
		return
	}

	// Verify OTP
	storedOTP := session.Get("reset_otp").(string)
	if storedOTP != req.OTP {
		utils.BadRequest(c, "Invalid OTP", "The OTP you entered is incorrect")
		return
	}

	// Generate a temporary token for password reset
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"exp":   time.Now().Add(15 * time.Minute).Unix(), // Token expires in 15 minutes
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		utils.InternalServerError(c, "Failed to generate reset token", "An error occurred while generating the reset token. Please try again later.")
		return
	}

	// Store token in session before clearing
	session.Set("reset_token", tokenString)
	if err := session.Save(); err != nil {
		log.Printf("Failed to save session: %v", err)
		utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
		return
	}

	utils.Success(c, "OTP verified successfully", gin.H{
		"message":    "Please reset your password",
		"token":      tokenString,
		"expires_in": 900, // Token expires in 15 minutes (900 seconds)
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
		utils.BadRequest(c, "Invalid request format", "Please provide new password and confirm password")
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		utils.BadRequest(c, "Invalid password", msg)
		return
	}

	// Validate confirm password
	if valid, msg := utils.ValidateConfirmPassword(req.NewPassword, req.ConfirmPassword); !valid {
		utils.BadRequest(c, "Password mismatch", msg)
		return
	}

	// Get token from session
	session := sessions.Default(c)
	tokenString := session.Get("reset_token")
	if tokenString == nil {
		utils.Unauthorized(c, "Invalid request: Please verify your OTP first")
		return
	}

	// Verify token
	token, err := jwt.Parse(tokenString.(string), func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		utils.Unauthorized(c, "Invalid or expired token, Your password reset session has expired. Please request a new password reset.")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		utils.Unauthorized(c, "Invalid token: Invalid password reset token")
		return
	}

	email := claims["email"].(string)

	// Get user from database
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		utils.NotFound(c, "User not found: No account exists with this email address")
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.NewPassword)); err == nil {
		utils.BadRequest(c, "Invalid password", "New password cannot be the same as current password")
		return
	}

	// Check password history (last 3 passwords)
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				utils.BadRequest(c, "Invalid password", "This password has been used recently. Please choose a different password")
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.InternalServerError(c, "Failed to process password", "An error occurred while securing your password. Please try again later.")
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.InternalServerError(c, "Failed to start transaction", "An error occurred while processing your request. Please try again later.")
		return
	}

	// Update user's password
	if err := tx.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update password", "An error occurred while updating your password. Please try again later.")
		return
	}

	// Add to password history
	passwordHistoryEntry := models.PasswordHistory{
		UserID:   user.ID,
		Password: string(hashedPassword),
	}
	if err := tx.Create(&passwordHistoryEntry).Error; err != nil {
		tx.Rollback()
		utils.InternalServerError(c, "Failed to update password history", "An error occurred while updating password history. Please try again later.")
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.InternalServerError(c, "Failed to commit changes", "An error occurred while saving your changes. Please try again later.")
		return
	}

	// Clear session
	session.Clear()
	session.Save()

	utils.Success(c, "Password reset successfully", gin.H{
		"redirect": gin.H{
			"url":     "/login",
			"message": "Please login with your new password",
		},
	})
}

func UserLogout(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()

	utils.Success(c, "Logout successful", nil)
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
