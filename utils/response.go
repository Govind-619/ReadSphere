package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// StandardResponse represents the standard API response structure
type StandardResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success sends a standardized success response
func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, StandardResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// Created sends a standardized created response (201)
func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, StandardResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	})
}

// SuccessWithPagination sends a paginated success response
func SuccessWithPagination(c *gin.Context, message string, data interface{}, total int64, page, perPage int) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "success",
		"message": message,
		"data":    data,
		"pagination": gin.H{
			"total":       total,
			"page":        page,
			"per_page":    perPage,
			"total_pages": (total + int64(perPage) - 1) / int64(perPage),
		},
	})
}

// Error sends a standardized error response
func Error(c *gin.Context, statusCode int, message string, err interface{}) {
	response := StandardResponse{
		Status:  "error",
		Message: message,
	}
	if err != nil {
		response.Data = gin.H{"error": err}
	}
	c.JSON(statusCode, response)
}

// BadRequest sends a 400 Bad Request response
func BadRequest(c *gin.Context, message string, err interface{}) {
	Error(c, http.StatusBadRequest, message, err)
}

// Unauthorized sends a 401 Unauthorized response
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message, nil)
}

// Forbidden sends a 403 Forbidden response
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message, nil)
}

// NotFound sends a 404 Not Found response
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message, nil)
}

// InternalServerError sends a 500 Internal Server Error response
func InternalServerError(c *gin.Context, message string, err interface{}) {
	Error(c, http.StatusInternalServerError, message, err)
}

// ValidationError sends a 422 Unprocessable Entity response
func ValidationError(c *gin.Context, message string, err interface{}) {
	Error(c, http.StatusUnprocessableEntity, message, err)
}

// Conflict sends a 409 Conflict response
func Conflict(c *gin.Context, message string, err interface{}) {
	Error(c, http.StatusConflict, message, err)
}

// Found sends a 302 Found response (for redirects)
func Found(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusFound, StandardResponse{
		Status:  "redirect",
		Message: message,
		Data:    data,
	})
}
