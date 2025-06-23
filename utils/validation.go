package utils

import (
	"fmt"

	"html"
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

// SanitizeString removes potentially dangerous characters and HTML tags
func SanitizeString(input string) string {
	// First, escape HTML special characters
	sanitized := html.EscapeString(input)

	// Remove any remaining HTML tags
	htmlTagRegex := regexp.MustCompile(`<[^>]*>`)
	sanitized = htmlTagRegex.ReplaceAllString(sanitized, "")

	// Remove JavaScript event handlers
	jsEventRegex := regexp.MustCompile(`on\w+="[^"]*"`)
	sanitized = jsEventRegex.ReplaceAllString(sanitized, "")

	// Remove data URIs
	dataUriRegex := regexp.MustCompile(`data:[^;]+;base64,[^"']+`)
	sanitized = dataUriRegex.ReplaceAllString(sanitized, "")

	return sanitized
}

// ValidateSQLInjection checks for common SQL injection patterns
func ValidateSQLInjection(input string) (bool, string) {
	// Common SQL injection patterns
	sqlInjectionPatterns := map[string]string{
		`(?i)(union\s+select)`:       "SQL injection detected: 'UNION SELECT' pattern found",
		`(?i)(union\s+all\s+select)`: "SQL injection detected: 'UNION ALL SELECT' pattern found",
		`(?i)(insert\s+into)`:        "SQL injection detected: 'INSERT INTO' pattern found",
		`(?i)(delete\s+from)`:        "SQL injection detected: 'DELETE FROM' pattern found",
		`(?i)(drop\s+table)`:         "SQL injection detected: 'DROP TABLE' pattern found",
		`(?i)(--\s*$)`:               "SQL injection detected: SQL comment found",
		`(?i)(/\*.*\*/)`:             "SQL injection detected: SQL comment block found",
		`(?i)(xp_cmdshell)`:          "SQL injection detected: 'xp_cmdshell' command found",
		`(?i)(exec\s*\()`:            "SQL injection detected: 'EXEC' command found",
		`(?i)(waitfor\s+delay)`:      "SQL injection detected: 'WAITFOR DELAY' command found",
		`(?i)(;.*$)`:                 "SQL injection detected: Multiple SQL statements detected",
	}

	for pattern, message := range sqlInjectionPatterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return false, message
		}
	}
	return true, ""
}

// ValidateXSS checks for common XSS attack patterns
func ValidateXSS(input string) (bool, string) {
	// Common XSS patterns
	xssPatterns := map[string]string{
		`(?i)(<script.*>)`:       "XSS detected: Script tag found",
		`(?i)(javascript:)`:      "XSS detected: JavaScript protocol found",
		`(?i)(vbscript:)`:        "XSS detected: VBScript protocol found",
		`(?i)(onload=)`:          "XSS detected: onload event handler found",
		`(?i)(onerror=)`:         "XSS detected: onerror event handler found",
		`(?i)(onclick=)`:         "XSS detected: onclick event handler found",
		`(?i)(eval\()`:           "XSS detected: eval function found",
		`(?i)(document\.cookie)`: "XSS detected: document.cookie access found",
		`(?i)(document\.write)`:  "XSS detected: document.write found",
		`(?i)(window\.location)`: "XSS detected: window.location manipulation found",
		`(?i)(alert\()`:          "XSS detected: alert function found",
	}

	for pattern, message := range xssPatterns {
		if matched, _ := regexp.MatchString(pattern, input); matched {
			return false, message
		}
	}
	return true, ""
}

// ValidateUsername checks if the username meets the requirements and is safe
func ValidateUsername(username string) (bool, string) {
	// First check for SQL injection and XSS
	if valid, msg := ValidateSQLInjection(username); !valid {
		return false, "Username: " + msg
	}
	if valid, msg := ValidateXSS(username); !valid {
		return false, "Username: " + msg
	}

	// Sanitize the input
	username = SanitizeString(username)

	if len(username) < 3 {
		return false, "Username must be at least 3 characters long"
	}
	if len(username) > 20 {
		return false, "Username must not exceed 20 characters"
	}
	if !usernameRegex.MatchString(username) {
		return false, "Username can only contain letters, numbers, and underscores"
	}
	return true, ""
}

// ValidateEmail checks if the email is valid and safe
func ValidateEmail(email string) (bool, string) {
	// First check for SQL injection and XSS
	if valid, msg := ValidateSQLInjection(email); !valid {
		return false, "Email: " + msg
	}
	if valid, msg := ValidateXSS(email); !valid {
		return false, "Email: " + msg
	}

	// Sanitize the input
	email = SanitizeString(email)

	if !emailRegex.MatchString(email) {
		return false, "Invalid email format. Please enter a valid email address"
	}
	return true, ""
}

// ValidatePassword checks if the password meets the requirements and is safe
func ValidatePassword(password string) (bool, string) {
	// First check for SQL injection and XSS
	if valid, msg := ValidateSQLInjection(password); !valid {
		return false, "Password: " + msg
	}
	if valid, msg := ValidateXSS(password); !valid {
		return false, "Password: " + msg
	}

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

// FormatPhoneNumber formats and validates an Indian phone number
func FormatPhoneNumber(phone string) (string, error) {
	// Remove all non-digit characters
	phone = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	// Remove leading '0' or '+91' if present
	if strings.HasPrefix(phone, "0") {
		phone = phone[1:]
	}
	if strings.HasPrefix(phone, "91") {
		phone = phone[2:]
	}

	// Check if the number is exactly 10 digits
	if len(phone) != 10 {
		return "", fmt.Errorf("phone number must be exactly 10 digits")
	}

	// Check if the number starts with a valid digit (6-9)
	if phone[0] < '6' || phone[0] > '9' {
		return "", fmt.Errorf("phone number must start with 6, 7, 8, or 9")
	}

	return phone, nil
}

// ValidatePhone checks if the phone number is valid
func ValidatePhone(phone string) (bool, string) {
	if phone == "" {
		return true, "" // Phone is optional
	}

	// First check for SQL injection and XSS
	if valid, msg := ValidateSQLInjection(phone); !valid {
		return false, "Phone: " + msg
	}
	if valid, msg := ValidateXSS(phone); !valid {
		return false, "Phone: " + msg
	}

	// Format and validate the phone number
	formattedPhone, err := FormatPhoneNumber(phone)
	if err != nil {
		return false, err.Error()
	}

	return true, formattedPhone
}

// ValidateName checks if the name is valid and safe
func ValidateName(name string) (bool, string) {
	if name == "" {
		return true, "" // Name is optional
	}

	// First check for SQL injection and XSS
	if valid, msg := ValidateSQLInjection(name); !valid {
		return false, "Name: " + msg
	}
	if valid, msg := ValidateXSS(name); !valid {
		return false, "Name: " + msg
	}

	// Sanitize the input
	name = SanitizeString(name)

	if len(strings.TrimSpace(name)) < 2 {
		return false, "Name must be at least 2 characters long"
	}

	// Check for numbers and special characters
	if matched, _ := regexp.MatchString(`[0-9!@#$%^&*(),.?":{}|<>]`, name); matched {
		return false, "Name cannot contain numbers or special characters"
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

// ValidateCouponValue checks if the coupon value is valid based on its type
func ValidateCouponValue(couponType string, value float64) error {
	if couponType == "percent" && value > 100 {
		return fmt.Errorf("percentage coupon value cannot exceed 100")
	}
	return nil
}
