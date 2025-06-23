package controllers

import (
	"log"
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
		utils.LogError("Registration attempt failed - Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", "Please check your input data and ensure all required fields are provided correctly.")
		return
	}

	utils.LogInfo("Registration attempt for email: %s, username: %s", req.Email, req.Username)

	// Validate username
	if valid, msg := utils.ValidateUsername(req.Username); !valid {
		utils.LogError("Registration attempt failed - Invalid username: %s - %s", req.Username, msg)
		utils.BadRequest(c, "Invalid username", msg)
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.LogError("Registration attempt failed - Invalid email: %s - %s", req.Email, msg)
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Validate password
	if valid, msg := utils.ValidatePassword(req.Password); !valid {
		utils.LogError("Registration attempt failed - Invalid password for email: %s - %s", req.Email, msg)
		utils.BadRequest(c, "Invalid password", msg)
		return
	}

	// Confirm password match
	if req.Password != req.ConfirmPassword {
		utils.LogError("Registration attempt failed - Passwords do not match for email: %s", req.Email)
		utils.BadRequest(c, "Passwords do not match", "Password and confirm password must be the same.")
		return
	}

	// Validate first name if provided
	if req.FirstName != "" {
		if valid, msg := utils.ValidateName(req.FirstName); !valid {
			utils.LogError("Registration attempt failed - Invalid first name: %s - %s", req.FirstName, msg)
			utils.BadRequest(c, "Invalid first name", msg)
			return
		}
	}

	// Validate last name if provided
	if req.LastName != "" {
		if valid, msg := utils.ValidateName(req.LastName); !valid {
			utils.LogError("Registration attempt failed - Invalid last name: %s - %s", req.LastName, msg)
			utils.BadRequest(c, "Invalid last name", msg)
			return
		}
	}

	// Validate and format phone if provided
	if req.Phone != "" {
		valid, formattedPhone := utils.ValidatePhone(req.Phone)
		if !valid {
			utils.LogError("Registration attempt failed - Invalid phone: %s - %s", req.Phone, formattedPhone)
			utils.BadRequest(c, "Invalid phone", formattedPhone)
			return
		}
		req.Phone = formattedPhone
	}

	// Check for SQL injection in all fields
	if valid, msg := utils.ValidateSQLInjection(req.Username); !valid {
		utils.LogError("Registration attempt failed - SQL injection attempt detected in username: %s", req.Username)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Email); !valid {
		utils.LogError("Registration attempt failed - SQL injection attempt detected in email: %s", req.Email)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.FirstName); !valid {
		utils.LogError("Registration attempt failed - SQL injection attempt detected in first name: %s", req.FirstName)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.LastName); !valid {
		utils.LogError("Registration attempt failed - SQL injection attempt detected in last name: %s", req.LastName)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateSQLInjection(req.Phone); !valid {
		utils.LogError("Registration attempt failed - SQL injection attempt detected in phone: %s", req.Phone)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check for XSS in all fields
	if valid, msg := utils.ValidateXSS(req.Username); !valid {
		utils.LogError("Registration attempt failed - XSS attempt detected in username: %s", req.Username)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.Email); !valid {
		utils.LogError("Registration attempt failed - XSS attempt detected in email: %s", req.Email)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.FirstName); !valid {
		utils.LogError("Registration attempt failed - XSS attempt detected in first name: %s", req.FirstName)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.LastName); !valid {
		utils.LogError("Registration attempt failed - XSS attempt detected in last name: %s", req.LastName)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}
	if valid, msg := utils.ValidateXSS(req.Phone); !valid {
		utils.LogError("Registration attempt failed - XSS attempt detected in phone: %s", req.Phone)
		utils.BadRequest(c, "Invalid input", msg)
		return
	}

	// Check if username already exists
	var existingUser models.User
	if err := config.DB.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		utils.LogError("Registration attempt failed - Username already exists: %s", req.Username)
		utils.Conflict(c, "Username already exists", "The username you've chosen is already taken. Please choose a different username.")
		return
	}

	// Check if email already exists
	if err := config.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		utils.LogError("Registration attempt failed - Email already exists: %s", req.Email)
		utils.Conflict(c, "Email already exists", "An account with this email address already exists. Please use a different email or try logging in.")
		return
	}

	// Check if phone already exists
	if req.Phone != "" {
		if err := config.DB.Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
			utils.LogError("Registration attempt failed - Phone already exists: %s", req.Phone)
			utils.Conflict(c, "Phone number already exists", "An account with this phone number already exists. Please use a different phone number or try logging in.")
			return
		}
	}

	//Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.LogError("Registration attempt failed - Password hashing error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to process password", "An error occurred while securing your password. Please try again later.")
		return
	}

	// Generate OTP and expiry
	otp := generateOTP()
	log.Printf("Registration OTP for %s: %s", req.Email, otp)
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
		utils.LogError("Registration attempt failed - JWT generation error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to generate registration token", err.Error())
		return
	}

	// Store OTP and expiry in session
	session := sessions.Default(c)
	session.Set("registration_otp", otp)
	session.Set("registration_otp_expires", otpExpiry)
	session.Set("registration_email", req.Email)
	if err := session.Save(); err != nil {
		utils.LogError("Registration attempt failed - Session save error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to save session", err.Error())
		return
	}

	// Send OTP email
	utils.LogInfo("Sending registration OTP to email: %s", req.Email)
	if err := utils.SendOTP(req.Email, otp); err != nil {
		utils.LogError("Registration attempt failed - OTP email error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending your verification email. Please try again later.")
		return
	}

	utils.LogInfo("Registration OTP sent successfully to email: %s", req.Email)
	utils.Success(c, "OTP sent to your email. Please verify to complete registration.", gin.H{
		"registration_token": tokenString,
		"expires_in":         900,
	})
}
