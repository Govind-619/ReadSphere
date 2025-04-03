package utils

import (
	"fmt"
	"net/http"
)

// AppError represents an application error
type AppError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Err     error  `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

// Unwrap implements the unwrap interface
func (e *AppError) Unwrap() error {
	return e.Err
}

// NewAppError creates a new AppError
func NewAppError(code int, message string, err error) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

// BadRequestError creates a 400 Bad Request error
func BadRequestError(message string, err error) *AppError {
	return NewAppError(http.StatusBadRequest, message, err)
}

// UnauthorizedError creates a 401 Unauthorized error
func UnauthorizedError(message string, err error) *AppError {
	return NewAppError(http.StatusUnauthorized, message, err)
}

// ForbiddenError creates a 403 Forbidden error
func ForbiddenError(message string, err error) *AppError {
	return NewAppError(http.StatusForbidden, message, err)
}

// NotFoundError creates a 404 Not Found error
func NotFoundError(message string, err error) *AppError {
	return NewAppError(http.StatusNotFound, message, err)
}

// ConflictError creates a 409 Conflict error
func ConflictError(message string, err error) *AppError {
	return NewAppError(http.StatusConflict, message, err)
}

// ServiceUnavailableError creates a 503 Service Unavailable error
func ServiceUnavailableError(message string, err error) *AppError {
	return NewAppError(http.StatusServiceUnavailable, message, err)
}

// IsAppError checks if an error is an AppError
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError returns the AppError if the error is an AppError
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return nil
}

// WrapError wraps an error with additional context
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// IsNotFoundError checks if an error is a "not found" error
func IsNotFoundError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusNotFound
	}
	return false
}

// IsValidationError checks if an error is a validation error
func IsValidationError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusUnprocessableEntity
	}
	return false
}

// IsUnauthorizedError checks if an error is an unauthorized error
func IsUnauthorizedError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusUnauthorized
	}
	return false
}

// IsForbiddenError checks if an error is a forbidden error
func IsForbiddenError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusForbidden
	}
	return false
}

// IsBadRequestError checks if an error is a bad request error
func IsBadRequestError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusBadRequest
	}
	return false
}

// IsInternalServerError checks if an error is an internal server error
func IsInternalServerError(err error) bool {
	if appErr := GetAppError(err); appErr != nil {
		return appErr.Code == http.StatusInternalServerError
	}
	return false
}
