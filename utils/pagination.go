package utils

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

// Pagination represents pagination parameters
type Pagination struct {
	Page     int
	Limit    int
	Offset   int
	Total    int64
	LastPage int
}

// NewPagination creates a new Pagination instance from query parameters
func NewPagination(c *gin.Context) *Pagination {
	// Get page and limit from query parameters
	pageStr := c.DefaultQuery("page", "1")
	limitStr := c.DefaultQuery("limit", "10")

	// Convert to integers
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}

	// Calculate offset
	offset := (page - 1) * limit

	return &Pagination{
		Page:   page,
		Limit:  limit,
		Offset: offset,
	}
}

// SetTotal sets the total number of items and calculates the last page
func (p *Pagination) SetTotal(total int64) {
	p.Total = total
	if p.Limit > 0 {
		p.LastPage = int((total + int64(p.Limit) - 1) / int64(p.Limit))
	}
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Data        interface{} `json:"data"`
	Pagination  Pagination  `json:"pagination"`
	TotalItems  int64       `json:"total_items"`
	CurrentPage int         `json:"current_page"`
	LastPage    int         `json:"last_page"`
	PerPage     int         `json:"per_page"`
}

// NewPaginatedResponse creates a new PaginatedResponse
func NewPaginatedResponse(data interface{}, pagination *Pagination) *PaginatedResponse {
	return &PaginatedResponse{
		Data:        data,
		Pagination:  *pagination,
		TotalItems:  pagination.Total,
		CurrentPage: pagination.Page,
		LastPage:    pagination.LastPage,
		PerPage:     pagination.Limit,
	}
}

// SendPaginatedResponse sends a paginated response
func SendPaginatedResponse(c *gin.Context, data interface{}, pagination *Pagination) {
	response := NewPaginatedResponse(data, pagination)
	Success(c, "Success", response)
}
