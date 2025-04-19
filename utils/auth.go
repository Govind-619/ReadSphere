package utils

import (
	"crypto/rand"
	"math/big"
	"errors"
	"os"
	"time"

	"github.com/Govind-619/ReadSphere/models"
	"github.com/golang-jwt/jwt"
	"golang.org/x/crypto/bcrypt"
)

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a password against a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateToken creates a JWT token for a user
func GenerateToken(user *models.User) (string, error) {
	// Create the token
	token := jwt.New(jwt.SigningMethodHS256)

	// Set claims
	claims := token.Claims.(jwt.MapClaims)
	claims["user_id"] = user.ID
	claims["email"] = user.Email
	claims["exp"] = time.Now().Add(time.Hour * 24).Unix() // 24 hour expiration

	// Generate encoded token
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the user ID
func ValidateToken(tokenString string) (uint, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})

	if err != nil {
		return 0, err
	}

	// Check if the token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Get user ID from claims
		userID, ok := claims["user_id"].(float64)
		if !ok {
			return 0, errors.New("invalid user ID in token")
		}
		return uint(userID), nil
	}

	return 0, errors.New("invalid token")
}

// GenerateOTP creates a 6-digit OTP
func GenerateOTP() string {
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
