package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/Govind-619/ReadSphere/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

type GoogleUserInfo struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
}

func GoogleLogin(c *gin.Context) {
	url := config.GoogleOAuthConfig.AuthCodeURL("state")
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	if code == "" {
		utils.BadRequest(c, "No code provided", nil)
		return
	}

	token, err := config.GoogleOAuthConfig.Exchange(c, code)
	if err != nil {
		utils.InternalServerError(c, "Failed to exchange token", err.Error())
		return
	}

	// Get user info from Google
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		utils.InternalServerError(c, "Failed to get user info", err.Error())
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		utils.InternalServerError(c, "Failed to read response", err.Error())
		return
	}

	var googleUser GoogleUserInfo
	if err := json.Unmarshal(data, &googleUser); err != nil {
		utils.InternalServerError(c, "Failed to parse user info", err.Error())
		return
	}

	// Check if user exists
	var user models.User
	if err := config.DB.Where("email = ?", googleUser.Email).First(&user).Error; err != nil {
		// Create new user if doesn't exist
		user = models.User{
			Email:      googleUser.Email,
			FirstName:  googleUser.GivenName,
			LastName:   googleUser.FamilyName,
			IsVerified: true,
			GoogleID:   googleUser.ID,
			Username:   googleUser.Email, // Using email as username for Google users
		}

		// Generate a secure but shorter password for Google users
		// Using first 8 chars of GoogleID + timestamp in seconds
		password := googleUser.ID[:8] + fmt.Sprintf("%d", time.Now().Unix())
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			utils.InternalServerError(c, "Failed to hash password", err.Error())
			return
		}
		user.Password = string(hashedPassword)

		if err := config.DB.Create(&user).Error; err != nil {
			utils.InternalServerError(c, "Failed to create user", err.Error())
			return
		}
	}

	// Generate JWT token
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err := jwtToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		utils.InternalServerError(c, "Failed to generate token", err.Error())
		return
	}

	// Instead of returning JSON, redirect to frontend with token
	redirectURL := fmt.Sprintf("%s?token=%s&user=%s",
		os.Getenv("FRONTEND_URL"),
		url.QueryEscape(tokenString),
		url.QueryEscape(fmt.Sprintf(`{"id":%d,"email":"%s","firstName":"%s","lastName":"%s"}`,
			user.ID, user.Email, user.FirstName, user.LastName)))

	c.Redirect(http.StatusTemporaryRedirect, redirectURL)
}
