# 📦 Project Structure

```
ReadSphereMVC/
├── config/           # Configuration and initialization
│   ├── config.go    # Main configuration
│   ├── database.go  # Database configuration
│   └── oauth.go     # OAuth settings
├── controllers/     # Business logic and request handling
│   ├── admin_*.go   # Admin controllers (dashboard, orders, inventory)
│   ├── user_*.go    # User controllers (auth, profile, password)
│   ├── book_*.go    # Book-related controllers
│   ├── cart_*.go    # Shopping cart controllers
│   ├── order_*.go   # Order management
│   └── wallet_*.go  # Wallet operations
├── middleware/      # Authentication and request middleware
├── models/          # Database models and schema
├── routes/          # API route definitions
├── utils/           # Helper functions and utilities
├── uploads/         # File storage for images
├── scripts/        # Deployment and maintenance scripts
├── go.mod          # Go module definition
├── go.sum          # Go module checksums
├── main.go         # Application entry point
└── Makefile        # Build and deployment commands

Note: The `logs/` directory is automatically created by the application to store log files
and should not be committed to version control.
```

## Directory Descriptions

### `config/`
Contains all configuration files including database settings, OAuth configuration, and application settings.

### `controllers/`
Business logic layer containing all request handlers organized by functionality:
- **Admin controllers**: Dashboard, user management, inventory, orders
- **User controllers**: Authentication, profile management, password operations
- **Book controllers**: CRUD operations, search, reviews
- **Cart controllers**: Shopping cart management
- **Order controllers**: Order processing and management
- **Wallet controllers**: Wallet operations and transactions

### `middleware/`
Authentication and request processing middleware for security and validation.

### `models/`
Database models and schema definitions using GORM.

### `routes/`
API route definitions organized by user type and functionality.

### `utils/`
Helper functions, utilities, and common functionality used across the application.

### `uploads/`
File storage directory for user uploads like profile images and book covers.

### `scripts/`
Deployment and maintenance scripts for database migrations and other operations. 