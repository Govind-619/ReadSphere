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
	ErrInvalidCredentials = "Invalid email or password. Please check your credentials and try again."
	ErrUserBlocked        = "Your account has been blocked. Please contact support for assistance."
	ErrInvalidToken       = "Your session has expired. Please log in again."
	ErrUnauthorized       = "You are not authorized to perform this action. Please log in with appropriate credentials."
	ErrForbidden          = "Access forbidden. You don't have permission to perform this action."

	// Validation errors
	ErrInvalidEmail      = "Invalid email format. Please enter a valid email address (e.g., user@example.com)."
	ErrInvalidPassword   = "Password must meet the following requirements:\n- At least 8 characters long\n- At least one uppercase letter\n- At least one lowercase letter\n- At least one number\n- At least one special character (@$!%*?&)"
	ErrInvalidPhone      = "Invalid phone number format. Please enter a valid phone number (e.g., +1234567890)."
	ErrInvalidPrice      = "Price must be greater than 0. Please enter a valid price."
	ErrInvalidStock      = "Stock quantity cannot be negative. Please enter a valid stock quantity."
	ErrInvalidRating     = "Rating must be between 1 and 5. Please enter a valid rating."
	ErrInvalidFileType   = "Invalid file type. Allowed types are: jpg, jpeg, png, gif. Please upload a valid image file."
	ErrFileTooLarge      = "File size exceeds the 5MB limit. Please upload a smaller file."
	ErrInvalidPagination = "Invalid pagination parameters. Page must be greater than 0 and limit between 1 and 100."

	// Database errors
	ErrRecordNotFound = "The requested record was not found. Please check the ID and try again."
	ErrDuplicateEntry = "A record with this information already exists. Please use different information."
	ErrDBConnection   = "Unable to connect to the database. Please try again later or contact support if the issue persists."

	// Server errors
	ErrInternalServer     = "An internal server error occurred. Please try again later or contact support if the issue persists."
	ErrServiceUnavailable = "The service is temporarily unavailable. Please try again later."
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
