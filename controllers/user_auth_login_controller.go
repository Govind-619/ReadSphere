package controllers

import (
	"os"
	"strings"
	"time"

	"log"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// LoginRequest represents the login request body
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginUser handles user login
func LoginUser(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Login attempt failed - Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid email or password", err.Error())
		return
	}

	// Sanitize input
	req.Email = utils.SanitizeString(req.Email)

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.LogError("Login attempt failed - Invalid email format: %s", req.Email)
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Check for SQL injection
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		utils.LogError("Login attempt failed - SQL injection attempt detected: %s", req.Email)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check for XSS
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		utils.LogError("Login attempt failed - XSS attempt detected: %s", req.Email)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.LogError("Login attempt failed - User not found: %s", req.Email)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		utils.LogError("Login attempt failed - Invalid password for user: %s", req.Email)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

	if user.IsBlocked {
		utils.LogError("Login attempt failed - Blocked account: %s", req.Email)
		utils.Forbidden(c, "Account is blocked")
		return
	}

	// Update last login
	user.LastLoginAt = time.Now()
	if err := config.DB.Save(&user).Error; err != nil {
		utils.LogError("Failed to update last login time for user: %s", req.Email)
	}

	// Generate JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.Model.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		utils.LogError("Failed to generate JWT token for user: %s", req.Email)
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}

	utils.LogInfo("User logged in successfully: %s", req.Email)
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
		utils.LogError("OTP verification failed - Invalid request format: %v", err)
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
		utils.LogError("OTP verification failed - Registration token missing")
		utils.BadRequest(c, "Registration token missing", "Registration token not provided")
		return
	}

	// Parse the registration JWT
	token, err := jwt.Parse(regToken, func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if err != nil || !token.Valid {
		utils.LogError("OTP verification failed - Invalid or expired registration token: %v", err)
		utils.Unauthorized(c, "Invalid or expired registration token")
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		utils.LogError("OTP verification failed - Invalid token claims")
		utils.Unauthorized(c, "Invalid token claims")
		return
	}

	email := claims["email"].(string)
	utils.LogInfo("OTP verification attempt for email: %s", email)

	// Use session to get OTP and expiry
	session := sessions.Default(c)
	sessEmail := session.Get("registration_email")
	if sessEmail == nil || sessEmail.(string) != email {
		utils.LogError("OTP verification failed - Session expired or email mismatch for: %s", email)
		utils.BadRequest(c, "Session expired", "Session expired or email mismatch. Please register again.")
		return
	}
	storedOTP := session.Get("registration_otp")
	otpExpires := session.Get("registration_otp_expires")
	if storedOTP == nil || otpExpires == nil {
		utils.LogError("OTP verification failed - OTP session expired for: %s", email)
		utils.BadRequest(c, "OTP session expired", "OTP session expired. Please register again.")
		return
	}
	if time.Now().Unix() > otpExpires.(int64) {
		// Check if registration token is still valid
		regExpires := int64(claims["exp"].(float64))
		if time.Now().Unix() > regExpires {
			utils.LogError("OTP verification failed - Registration expired for: %s", email)
			utils.BadRequest(c, "Registration expired", "Registration expired. Please register again.")
			return
		}
		// Resend new OTP
		newOTP := generateOTP()
		log.Printf("Registration OTP for %s: %s", email, newOTP)
		utils.LogInfo("Resending OTP to: %s", email)
		session.Set("registration_otp", newOTP)
		session.Set("registration_otp_expires", time.Now().Add(1*time.Minute).Unix())
		if err := session.Save(); err != nil {
			utils.LogError("Failed to save session for OTP resend: %v", err)
			utils.InternalServerError(c, "Failed to save session", err.Error())
			return
		}
		if err := utils.SendOTP(email, newOTP); err != nil {
			utils.LogError("Failed to resend OTP to: %s", email)
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
		utils.LogError("OTP verification failed - Invalid OTP for: %s", email)
		utils.BadRequest(c, "Invalid OTP", "The OTP you entered is incorrect")
		return
	}

	// Check if user already exists
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err == nil {
		utils.LogError("OTP verification failed - User already exists: %s", email)
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
		utils.LogError("Failed to create user account: %s", email)
		utils.InternalServerError(c, "Failed to create user account", err.Error())
		return
	}

	// Create referral code for the new user
	_, err = GetOrCreateUserReferralCode(user.ID)
	if err != nil {
		utils.LogError("Failed to create referral code for new user: %s - %v", email, err)
		// Don't fail registration if referral code creation fails
	}

	// Process referral code if present in registration claims
	if referralCode, ok := claims["referral_code"].(string); ok && referralCode != "" {
		err := UseReferralCode(referralCode, user.ID)
		if err != nil {
			utils.LogError("Failed to process referral code for user: %s", email)
			utils.BadRequest(c, "Invalid referral code", "Invalid or expired referral code")
			return
		}
	}

	utils.LogInfo("User registration completed successfully: %s", email)
	utils.Success(c, "Email verified and registration completed successfully", gin.H{
		"redirect": gin.H{
			"url":     "/login",
			"message": "Please login with your credentials",
		},
	})
}
