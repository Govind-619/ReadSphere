package utils

// Application constants
const (
	// Application name
	AppName = "ReadSphere"

	// API version
	APIVersion = "v1"

	// Default port
	DefaultPort = "8080"

	// Default database host
	DefaultDBHost = "localhost"

	// Default database port
	DefaultDBPort = "5432"

	// Default database name
	DefaultDBName = "readsphere"

	// Default database user
	DefaultDBUser = "postgres"

	// Default database password
	DefaultDBPassword = "postgres"

	// JWT token expiration (24 hours)
	JWTExpiration = "24h"

	// OTP expiration (10 minutes)
	OTPExpiration = "10m"

	// Password reset token expiration (1 hour)
	PasswordResetExpiration = "1h"

	// Maximum file size for uploads (5MB)
	MaxFileSize = 5 * 1024 * 1024

	// Default pagination limit
	DefaultPaginationLimit = 10

	// Maximum pagination limit
	MaxPaginationLimit = 100

	// Minimum password length
	MinPasswordLength = 8

	// Maximum password length
	MaxPasswordLength = 32

	// Minimum name length
	MinNameLength = 2

	// Maximum name length
	MaxNameLength = 50

	// Minimum description length
	MinDescriptionLength = 10

	// Maximum description length
	MaxDescriptionLength = 500

	// Minimum rating
	MinRating = 1

	// Maximum rating
	MaxRating = 5
)

// Error messages
const (
	// Authentication errors
	ErrInvalidCredentials = "Invalid email or password"
	ErrUserBlocked        = "Your account has been blocked"
	ErrInvalidToken       = "Invalid or expired token"
	ErrUnauthorized       = "Unauthorized access"
	ErrForbidden          = "Access forbidden"

	// Validation errors
	ErrInvalidEmail      = "Invalid email format"
	ErrInvalidPassword   = "Password must be at least 8 characters long and contain at least one uppercase letter, one lowercase letter, one number, and one special character"
	ErrInvalidPhone      = "Invalid phone number format"
	ErrInvalidPrice      = "Price must be greater than 0"
	ErrInvalidStock      = "Stock cannot be negative"
	ErrInvalidRating     = "Rating must be between 1 and 5"
	ErrInvalidFileType   = "Invalid file type. Allowed types: jpg, jpeg, png, gif"
	ErrFileTooLarge      = "File size exceeds 5MB limit"
	ErrInvalidPagination = "Invalid pagination parameters"

	// Database errors
	ErrRecordNotFound = "Record not found"
	ErrDuplicateEntry = "Duplicate entry"
	ErrDBConnection   = "Database connection error"

	// Server errors
	ErrInternalServer     = "Internal server error"
	ErrServiceUnavailable = "Service unavailable"
)

// Success messages
const (
	// Authentication messages
	MsgLoginSuccess    = "Login successful"
	MsgLogoutSuccess   = "Logout successful"
	MsgRegisterSuccess = "Registration successful"
	MsgOTPSent         = "OTP sent successfully"
	MsgOTPVerified     = "OTP verified successfully"
	MsgPasswordReset   = "Password reset successful"

	// CRUD operation messages
	MsgCreateSuccess  = "Created successfully"
	MsgUpdateSuccess  = "Updated successfully"
	MsgDeleteSuccess  = "Deleted successfully"
	MsgBlockSuccess   = "Blocked successfully"
	MsgUnblockSuccess = "Unblocked successfully"
	MsgUploadSuccess  = "File uploaded successfully"
)
