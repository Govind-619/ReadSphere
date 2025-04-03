package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// FieldValidationError represents a validation error for a specific field
type FieldValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// FieldValidationErrors represents multiple field validation errors
type FieldValidationErrors []FieldValidationError

// Error implements the error interface
func (e FieldValidationErrors) Error() string {
	var messages []string
	for _, err := range e {
		messages = append(messages, fmt.Sprintf("%s: %s", err.Field, err.Message))
	}
	return strings.Join(messages, "; ")
}

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]{3,20}$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	phoneRegex    = regexp.MustCompile(`^\+?[1-9]\d{1,14}$`)
	// Password validation regex patterns
	hasLower   = regexp.MustCompile(`[a-z]`)
	hasUpper   = regexp.MustCompile(`[A-Z]`)
	hasNumber  = regexp.MustCompile(`[0-9]`)
	hasSpecial = regexp.MustCompile(`[@$!%*?&]`)
	validChars = regexp.MustCompile(`^[A-Za-z\d@$!%*?&]+$`)
)

// ValidateUsername checks if the username meets the requirements
func ValidateUsername(username string) (bool, string) {
	if len(username) < 3 || len(username) > 20 {
		return false, "Username must be between 3 and 20 characters"
	}
	if !usernameRegex.MatchString(username) {
		return false, "Username can only contain letters, numbers, and underscores"
	}
	return true, ""
}

// ValidateEmail checks if the email is valid
func ValidateEmail(email string) (bool, string) {
	if !emailRegex.MatchString(email) {
		return false, "Invalid email format"
	}
	return true, ""
}

// ValidatePassword checks if the password meets the requirements
func ValidatePassword(password string) (bool, string) {
	if len(password) < 8 {
		return false, "Password must be at least 8 characters long"
	}

	if !hasLower.MatchString(password) {
		return false, "Password must contain at least one lowercase letter"
	}

	if !hasUpper.MatchString(password) {
		return false, "Password must contain at least one uppercase letter"
	}

	if !hasNumber.MatchString(password) {
		return false, "Password must contain at least one number"
	}

	if !hasSpecial.MatchString(password) {
		return false, "Password must contain at least one special character (@$!%*?&)"
	}

	if !validChars.MatchString(password) {
		return false, "Password can only contain letters, numbers, and special characters (@$!%*?&)"
	}

	return true, ""
}

// ValidatePhone checks if the phone number is valid
func ValidatePhone(phone string) (bool, string) {
	if phone == "" {
		return true, "" // Phone is optional
	}
	if !phoneRegex.MatchString(phone) {
		return false, "Invalid phone number format"
	}
	return true, ""
}

// ValidateName checks if the name is valid
func ValidateName(name string) (bool, string) {
	if name == "" {
		return true, "" // Name is optional
	}
	if len(strings.TrimSpace(name)) < 2 {
		return false, "Name must be at least 2 characters long"
	}
	return true, ""
}

// ValidateConfirmPassword checks if the confirm password matches the password
func ValidateConfirmPassword(password, confirmPassword string) (bool, string) {
	if password != confirmPassword {
		return false, "Passwords do not match"
	}
	return true, ""
}

// ValidatePrice validates a price
func ValidatePrice(price float64) error {
	if price <= 0 {
		return fmt.Errorf("price must be greater than 0")
	}
	return nil
}

// ValidateStock validates stock quantity
func ValidateStock(stock int) error {
	if stock < 0 {
		return fmt.Errorf("stock cannot be negative")
	}
	return nil
}

// ValidateRating validates a product rating
func ValidateRating(rating int) error {
	if rating < 1 || rating > 5 {
		return fmt.Errorf("rating must be between 1 and 5")
	}
	return nil
}

// ValidateStringLength validates string length
func ValidateStringLength(str string, min, max int) error {
	length := len(strings.TrimSpace(str))
	if length < min {
		return fmt.Errorf("must be at least %d characters long", min)
	}
	if length > max {
		return fmt.Errorf("must not exceed %d characters", max)
	}
	return nil
}
