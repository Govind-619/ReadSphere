# ReadSphere Backend API

A full-featured backend API for an e-commerce platform built with Go, Gin, PostgreSQL, and GORM.

## Features

### Admin Side
- Secure admin authentication
- User management (block/unblock users)
- Category management (CRUD operations)
- Product management (CRUD operations with image uploads)
- Search and pagination functionality

### User Side
- User authentication (signup, login, OTP verification)
- Product browsing with advanced filtering and sorting
- Product details with reviews and ratings
- Shopping cart functionality

## Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Make (optional, for using Makefile commands)

## Installation

1. Clone the repository:
```bash
git clone https://github.com/Govind-619/ReadSphere.git
cd ReadSphere
```

2. Install dependencies:
```bash
go mod download
```

3. Create a `.env` file in the root directory with the following content:
```env
DB=host=localhost user=postgres password=your_password dbname=readsphere port=5432 sslmode=disable
Admin_Email=admin@example.com
Admin_Password=your_admin_password
JWT_SECRET=your_jwt_secret
```

4. Create the database:
```bash
createdb readsphere
```

5. Run the application:
```bash
go run main.go
```

## API Endpoints

### Admin Routes
- `POST /api/v1/admin/login` - Admin login
- `GET /api/v1/admin/users` - Get users list
- `PATCH /api/v1/admin/users/:id/block` - Block user
- `PATCH /api/v1/admin/users/:id/unblock` - Unblock user
- `POST /api/v1/admin/categories` - Create category
- `GET /api/v1/admin/categories` - Get categories list
- `PUT /api/v1/admin/categories/:id` - Update category
- `DELETE /api/v1/admin/categories/:id` - Delete category
- `POST /api/v1/admin/products` - Create product
- `GET /api/v1/admin/products` - Get products list
- `PUT /api/v1/admin/products/:id` - Update product
- `DELETE /api/v1/admin/products/:id` - Delete product

### User Routes
- `POST /api/v1/user/register` - User registration
- `POST /api/v1/user/login` - User login
- `POST /api/v1/user/verify-otp` - Verify OTP
- `POST /api/v1/user/resend-otp` - Resend OTP
- `POST /api/v1/user/forgot-password` - Forgot password
- `POST /api/v1/user/reset-password` - Reset password
- `GET /api/v1/user/products` - Get products list
- `GET /api/v1/user/products/:id` - Get product details
- `POST /api/v1/user/products/:id/review` - Add product review

## Project Structure

```
ReadSphere/
├── config/
│   └── config.go         # Database configuration
├── controllers/
│   ├── admin_controller.go
│   ├── user_controller.go
│   ├── product_controller.go
│   └── category_controller.go
├── middleware/
│   └── auth.go          # Authentication middleware
├── models/
│   └── models.go        # Database models
├── uploads/             # Product images storage
├── .env                 # Environment variables
├── go.mod              # Go module file
├── go.sum              # Go module checksums
├── main.go             # Application entry point
└── README.md           # Project documentation
```

## Testing

The API can be tested using Postman. Import the following collection:

[Postman Collection Link]

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details. 