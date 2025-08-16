# 🛠️ Technical Stack

## Core
- Go 1.23.0 (with Go modules)
- Gin Web Framework 1.9.1
- GORM 1.25.8 with PostgreSQL driver
- PostgreSQL 12+

## Authentication & Security
- JWT (github.com/golang-jwt/jwt v3.2.2)
- Gin Sessions with secure cookie store
- Google OAuth2 integration
- Bcrypt password hashing (golang.org/x/crypto v0.31.0)

## Payment Processing
- Razorpay Go SDK v1.3.2
- Secure webhook handling
- HMAC signature verification

## Export & Reports
- PDF Generation (jung-kurt/gofpdf v1.16.2)
- Excel Export (tealeg/xlsx v1.0.5)

## Email & Communication
- SMTP integration (gopkg.in/gomail.v2)
- HTML email templates

## Development & Testing
- Unit testing with Go's testing package
- Testify for assertions and mocking (v1.9.0)
- Environment configuration via .env
- Structured logging with rotating files

## Utilities & Helpers
- UUID generation (github.com/google/uuid v1.6.0)
- Environment variable management (github.com/joho/godotenv v1.5.1)
- String manipulation and validation
- File upload handling with size limits

## DevOps
- Docker support
- Makefile for common operations
- CI/CD ready configuration
- Request ID middleware for tracing
- CORS middleware for cross-origin requests
- Security headers middleware 