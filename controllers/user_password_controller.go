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

// ForgotPasswordRequest represents the forgot password request body
type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

func ForgotPassword(c *gin.Context) {
	var req ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.LogError("Password reset attempt failed - Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", "Please provide a valid email address")
		return
	}

	// Validate email
	if valid, msg := utils.ValidateEmail(req.Email); !valid {
		utils.LogError("Password reset attempt failed - Invalid email: %s - %s", req.Email, msg)
		utils.BadRequest(c, "Invalid email", msg)
		return
	}

	// Check if user exists
	var user models.User
	if err := config.DB.Where("email = ?", req.Email).First(&user).Error; err != nil {
		utils.LogError("Password reset attempt failed - User not found: %s", req.Email)
		utils.NotFound(c, "User not found, No account exists with this email address")
		return
	}

	// Generate OTP
	otp := generateOTP()
	log.Println("Forgot Password OTP:", otp)
	utils.LogInfo("Password reset OTP generated for %s: %s", req.Email, otp)

	// Store email and OTP in session
	session := sessions.Default(c)
	session.Set("reset_email", req.Email)
	session.Set("reset_otp", otp)
	session.Set("reset_otp_expires", time.Now().Add(time.Minute*1).Unix())

	if err := session.Save(); err != nil {
		utils.LogError("Password reset attempt failed - Session save error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
		return
	}

	// Send OTP via email
	if err := utils.SendOTP(req.Email, otp); err != nil {
		utils.LogError("Password reset attempt failed - OTP email error for email: %s - %v", req.Email, err)
		utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending the verification email. Please try again later.")
		return
	}

	utils.LogInfo("Password reset OTP sent successfully to email: %s", req.Email)
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
		utils.LogError("Password reset OTP verification failed - Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", "Please provide OTP")
		return
	}

	// Get reset data from session
	session := sessions.Default(c)
	email := session.Get("reset_email")
	if email == nil {
		utils.LogError("Password reset OTP verification failed - No reset session found")
		utils.BadRequest(c, "Invalid request", "Please request password reset first")
		return
	}

	// Check if OTP has expired
	otpExpires := session.Get("reset_otp_expires").(int64)
	if time.Now().Unix() > otpExpires {
		// Generate new OTP
		newOTP := generateOTP()
		log.Println("Reset OTP expired, sending new OTP:", newOTP)
		utils.LogInfo("Password reset OTP expired, generating new OTP for %s: %s", email, newOTP)

		// Update session with new OTP and expiration time
		session.Set("reset_otp", newOTP)
		session.Set("reset_otp_expires", time.Now().Add(time.Minute*1).Unix())

		if err := session.Save(); err != nil {
			utils.LogError("Password reset OTP resend failed - Session save error for email: %s - %v", email, err)
			utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
			return
		}

		// Send new OTP via email
		if err := utils.SendOTP(email.(string), newOTP); err != nil {
			utils.LogError("Password reset OTP resend failed - Email error for email: %s - %v", email, err)
			utils.InternalServerError(c, "Failed to send verification email", "An error occurred while sending the verification email. Please try again later.")
			return
		}

		utils.LogInfo("New password reset OTP sent successfully to email: %s", email)
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
		utils.LogError("Password reset OTP verification failed - Invalid OTP for email: %s", email)
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
		utils.LogError("Password reset OTP verification failed - Token generation error for email: %s - %v", email, err)
		utils.InternalServerError(c, "Failed to generate reset token", "An error occurred while generating the reset token. Please try again later.")
		return
	}

	// Store token in session before clearing
	session.Set("reset_token", tokenString)
	if err := session.Save(); err != nil {
		utils.LogError("Password reset OTP verification failed - Session save error for email: %s - %v", email, err)
		utils.InternalServerError(c, "Failed to save session", "An error occurred while processing your request. Please try again later.")
		return
	}

	utils.LogInfo("Password reset OTP verified successfully for email: %s", email)
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
		utils.LogError("Password reset failed - Invalid request format: %v", err)
		utils.BadRequest(c, "Invalid request format", "Please provide new password and confirm password")
		return
	}

	// Validate new password
	if valid, msg := utils.ValidatePassword(req.NewPassword); !valid {
		utils.LogError("Password reset failed - Invalid password format: %s", msg)
		utils.BadRequest(c, "Invalid password", msg)
		return
	}

	// Validate confirm password
	if valid, msg := utils.ValidateConfirmPassword(req.NewPassword, req.ConfirmPassword); !valid {
		utils.LogError("Password reset failed - Password mismatch: %s", msg)
		utils.BadRequest(c, "Password mismatch", msg)
		return
	}

	// Get token from session
	session := sessions.Default(c)
	tokenString := session.Get("reset_token")
	if tokenString == nil {
		utils.LogError("Password reset failed - No reset token found in session")
		utils.Unauthorized(c, "Invalid request: Please verify your OTP first")
		return
	}

	// Verify token
	token, err := jwt.Parse(tokenString.(string), func(token *jwt.Token) (interface{}, error) {
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil || !token.Valid {
		utils.LogError("Password reset failed - Invalid or expired token: %v", err)
		utils.Unauthorized(c, "Invalid or expired token, Your password reset session has expired. Please request a new password reset.")
		return
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		utils.LogError("Password reset failed - Invalid token claims")
		utils.Unauthorized(c, "Invalid token: Invalid password reset token")
		return
	}

	email := claims["email"].(string)

	// Get user from database
	var user models.User
	if err := config.DB.Where("email = ?", email).First(&user).Error; err != nil {
		utils.LogError("Password reset failed - User not found: %s", email)
		utils.NotFound(c, "User not found: No account exists with this email address")
		return
	}

	// Check if new password is same as current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.NewPassword)); err == nil {
		utils.LogError("Password reset failed - New password same as current password for user: %s", email)
		utils.BadRequest(c, "Invalid password", "New password cannot be the same as current password")
		return
	}

	// Check password history (last 3 passwords)
	var passwordHistory []models.PasswordHistory
	if err := config.DB.Where("user_id = ?", user.ID).Order("created_at DESC").Limit(3).Find(&passwordHistory).Error; err == nil {
		for _, history := range passwordHistory {
			if err := bcrypt.CompareHashAndPassword([]byte(history.Password), []byte(req.NewPassword)); err == nil {
				utils.LogError("Password reset failed - Password recently used for user: %s", email)
				utils.BadRequest(c, "Invalid password", "This password has been used recently. Please choose a different password")
				return
			}
		}
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.LogError("Password reset failed - Password hashing error for user: %s - %v", email, err)
		utils.InternalServerError(c, "Failed to process password", "An error occurred while securing your password. Please try again later.")
		return
	}

	// Start transaction
	tx := config.DB.Begin()
	if tx.Error != nil {
		utils.LogError("Password reset failed - Transaction start error for user: %s - %v", email, tx.Error)
		utils.InternalServerError(c, "Failed to start transaction", "An error occurred while processing your request. Please try again later.")
		return
	}

	// Update user's password
	if err := tx.Model(&user).Update("password", string(hashedPassword)).Error; err != nil {
		tx.Rollback()
		utils.LogError("Password reset failed - Password update error for user: %s - %v", email, err)
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
		utils.LogError("Password reset failed - Password history update error for user: %s - %v", email, err)
		utils.InternalServerError(c, "Failed to update password history", "An error occurred while updating password history. Please try again later.")
		return
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		utils.LogError("Password reset failed - Transaction commit error for user: %s - %v", email, err)
		utils.InternalServerError(c, "Failed to commit changes", "An error occurred while saving your changes. Please try again later.")
		return
	}

	// Clear session
	session.Clear()
	session.Save()

	utils.LogInfo("Password reset completed successfully for user: %s", email)
	utils.Success(c, "Password reset successfully", gin.H{
		"redirect": gin.H{
			"url":     "/login",
			"message": "Please login with your new password",
		},
	})
}
