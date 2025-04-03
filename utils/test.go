package utils

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Govind-619/ReadSphere/config"
	"github.com/Govind-619/ReadSphere/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestSetup initializes test environment
func TestSetup(t *testing.T) {
	// Load test environment variables
	if _, err := config.LoadConfig(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Initialize test database
	config.InitDB()

	// Clear test data
	ClearTestData()
}

// TestTeardown cleans up test environment
func TestTeardown(t *testing.T) {
	// Clear test data
	ClearTestData()
}

// ClearTestData clears all test data from the database
func ClearTestData() {
	config.DB.Exec("TRUNCATE TABLE users CASCADE")
	config.DB.Exec("TRUNCATE TABLE admins CASCADE")
	config.DB.Exec("TRUNCATE TABLE categories CASCADE")
	config.DB.Exec("TRUNCATE TABLE products CASCADE")
	config.DB.Exec("TRUNCATE TABLE reviews CASCADE")
}

// CreateTestUser creates a test user
func CreateTestUser(t *testing.T) *models.User {
	user := &models.User{
		Email:     "test@example.com",
		Password:  "Test123!",
		FirstName: "Test",
		LastName:  "User",
		Phone:     "+1234567890",
	}

	if err := CreateUser(user); err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// CreateTestAdmin creates a test admin
func CreateTestAdmin(t *testing.T) *models.Admin {
	admin := &models.Admin{
		Email:    "admin@example.com",
		Password: "Admin123!",
	}

	if err := CreateAdmin(admin); err != nil {
		t.Fatalf("Failed to create test admin: %v", err)
	}

	return admin
}

// CreateTestCategory creates a test category
func CreateTestCategory(t *testing.T) *models.Category {
	category := &models.Category{
		Name:        "Test Category",
		Description: "Test Category Description",
	}

	if err := CreateCategory(category); err != nil {
		t.Fatalf("Failed to create test category: %v", err)
	}

	return category
}

// CreateTestProduct creates a test product
func CreateTestProduct(t *testing.T, categoryID uint) *models.Product {
	product := &models.Product{
		Name:        "Test Product",
		Description: "Test Product Description",
		Price:       99.99,
		Stock:       100,
		CategoryID:  categoryID,
		ImageURL:    "test.jpg",
	}

	if err := CreateProduct(product); err != nil {
		t.Fatalf("Failed to create test product: %v", err)
	}

	return product
}

// CreateTestReview creates a test review
func CreateTestReview(t *testing.T, userID, productID uint) *models.Review {
	review := &models.Review{
		UserID:    userID,
		ProductID: productID,
		Rating:    5,
		Comment:   "Test Review",
	}

	if err := CreateReview(review); err != nil {
		t.Fatalf("Failed to create test review: %v", err)
	}

	return review
}

// TestRequest represents a test HTTP request
type TestRequest struct {
	Method  string
	Path    string
	Body    interface{}
	Headers map[string]string
}

// TestResponse represents a test HTTP response
type TestResponse struct {
	StatusCode int
	Body       map[string]interface{}
}

// MakeTestRequest makes a test HTTP request
func MakeTestRequest(t *testing.T, router *gin.Engine, req TestRequest) TestResponse {
	// Create request body
	var body []byte
	if req.Body != nil {
		var err error
		body, err = json.Marshal(req.Body)
		if err != nil {
			t.Fatalf("Failed to marshal request body: %v", err)
		}
	}

	// Create HTTP request
	httpReq, err := http.NewRequest(req.Method, req.Path, bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Create response recorder
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httpReq)

	// Parse response body
	var responseBody map[string]interface{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &responseBody); err != nil {
			t.Fatalf("Failed to unmarshal response body: %v", err)
		}
	}

	return TestResponse{
		StatusCode: w.Code,
		Body:       responseBody,
	}
}

// AssertResponse asserts the test response
func AssertResponse(t *testing.T, response TestResponse, expectedStatusCode int, expectedBody map[string]interface{}) {
	assert.Equal(t, expectedStatusCode, response.StatusCode)
	if expectedBody != nil {
		assert.Equal(t, expectedBody, response.Body)
	}
}

// GetTestToken generates a test JWT token
func GetTestToken(t *testing.T, user *models.User) string {
	token, err := GenerateToken(user)
	if err != nil {
		t.Fatalf("Failed to generate test token: %v", err)
	}
	return token
}

// GetTestAdminToken generates a test admin JWT token
func GetTestAdminToken(t *testing.T, admin *models.Admin) string {
	// TODO: Implement admin token generation
	return "test-admin-token"
}
