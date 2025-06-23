package utils

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

// NewError creates a new error with a message
func NewError(message string) error {
	return errors.New(message)
}

// GetPaginationParams extracts pagination parameters from the request
func GetPaginationParams(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 10
	}

	return page, limit
}
