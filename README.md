# 📚 ReadSphere - Modern E-Commerce Bookstore API

![ReadSphere Logo](logo.png)

ReadSphere is a full-featured e-commerce platform for books, built with Go, Gin, PostgreSQL, and GORM. It offers a robust API with comprehensive features for both users and administrators.

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-12+-336791?style=flat&logo=postgresql)](https://www.postgresql.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## Table of Contents
- [✨ Key Features](#-key-features)
- [🛠️ Technical Stack](#️-technical-stack)
- [📦 Project Structure](#-project-structure)
- [🚀 API Endpoints](#-api-endpoints)
- [🚀 Getting Started](#-getting-started)
- [🤝 Contributing](#-contributing)
- [📝 License](#-license)
- [📧 Contact](#-contact)

## ✨ Key Features

### 👤 User Features

#### Authentication & Security
- Email/Password registration with OTP verification
- Google OAuth2 integration with callback handling
- Forgot password with secure reset token
- Session management with secure cookies
- Password history tracking for security

#### Shopping Experience
- Advanced book browsing with category/genre filters
- Dynamic search functionality
- Cart management with:
  - Stock validation on add/update
  - Automatic wishlist sync
  - Quantity limits based on stock
  - Price calculations with discounts
- Wishlist functionality with auto-sync
- Multiple payment options:
  - Cash on Delivery (orders ≤ ₹1000)
  - Razorpay integration
  - Wallet payments

#### Order Management
- Order placement with validation
- Multiple delivery addresses
- Order tracking and history
- Order cancellation with refund
- Return requests with reason
- PDF/Excel invoice generation
- Email notifications

#### Profile & Wallet
- Profile management with image upload
- Multiple address management
- Wallet system with:
  - Secure Razorpay top-up
  - Automatic refund credits
  - Transaction history
  - Balance tracking
- Referral system:
  - Unique referral codes
  - Reward coupons
  - Referral tracking

#### Offers & Discounts
- Coupon system with:
  - Percentage/fixed amount discounts
  - Minimum order value
  - Maximum discount caps
  - Usage limits
  - Expiry dates
- Category/Product level offers
- Referral reward coupons
- Automatic discount calculations

### 👨‍💼 Admin Features

#### Dashboard & Analytics
- Real-time sales overview
- Order statistics
- Customer insights
- Stock monitoring
- Sales reports:
  - Daily/Weekly/Monthly views
  - PDF/Excel export
  - Product performance
  - Revenue analytics

#### Product Management
- Book inventory management
- Category & genre management
- Multi-image upload support
- Stock level tracking
- Offer management:
  - Product-specific offers
  - Category-wide discounts
  - Time-bound promotions

#### Order Processing
- Order status management
- Return request handling
- Refund processing:
  - Automatic wallet credits
  - Return verification
- Order filtering and search
- Bulk order processing
- Invoice generation

#### User Management
- User account management
- Block/Unblock functionality
- Review moderation
- Coupon management:
  - Create/Edit/Delete coupons
  - Usage tracking
  - Validity management
- Referral oversight

## 🛠️ Technical Stack

### Core
- Go 1.24.0 (with Go modules)
- Gin Web Framework 1.9.1
- GORM 1.25.8 with PostgreSQL driver
- PostgreSQL 12+

### Authentication & Security
- JWT (github.com/golang-jwt/jwt v3.2.2)
- Gin Sessions with secure cookie store
- Google OAuth2 integration
- Bcrypt password hashing (golang.org/x/crypto)

### Payment Processing
- Razorpay Go SDK v1.3.2
- Secure webhook handling

### Export & Reports
- PDF Generation (jung-kurt/gofpdf v1.16.2)
- Excel Export (tealeg/xlsx v1.0.5)

### Email & Communication
- SMTP integration (gopkg.in/gomail.v2)
- HTML email templates

### Development & Testing
- Unit testing with Go's testing package
- Testify for assertions and mocking
- Environment configuration via .env
- Structured logging with rotating files

### DevOps
- Docker support
- Makefile for common operations
- CI/CD ready configuration

## 📦 Project Structure

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

## 🚀 API Endpoints

### 🔓 Public Endpoints

#### Authentication
- `GET /auth/google/login` - Google OAuth login
- `GET /auth/google/callback` - Google OAuth callback
- `POST /v1/register` - User registration
- `POST /v1/login` - User login
- `POST /v1/verify-otp` - OTP verification
- `POST /v1/forgot-password` - Password reset request
- `POST /v1/verify-reset-otp` - Verify reset OTP
- `POST /v1/reset-password` - Reset password

#### Books & Categories
- `GET /v1/books` - List all books
- `GET /v1/books/:id` - Get book details
- `GET /v1/books/:id/images` - Get book images
- `GET /v1/categories` - List categories
- `GET /v1/categories/:id/books` - Books by category

#### Referral System
- `GET /v1/referral/:token` - Get referral information
- `GET /v1/referral/invite/:token` - Accept referral invitation

### 🔒 Protected User Endpoints

#### Profile Management
- `GET /v1/profile` - Get user profile
- `PUT /v1/profile` - Update basic profile
- `PUT /v1/profile/email` - Update email
- `POST /v1/profile/email/verify` - Verify email update
- `PUT /v1/profile/password` - Change password
- `POST /v1/profile/image` - Upload profile image

#### Address Management
- `GET /v1/profile/address` - List addresses
- `POST /v1/profile/address` - Add address
- `PUT /v1/profile/address/:id` - Edit address
- `DELETE /v1/profile/address/:id` - Delete address
- `PUT /v1/profile/address/:id/default` - Set default address

#### Shopping Cart
- `POST /v1/user/cart/add` - Add to cart
- `GET /v1/user/cart` - View cart
- `PUT /v1/user/cart/update` - Update quantities
- `DELETE /v1/user/cart/remove` - Remove item
- `DELETE /v1/user/cart/clear` - Clear cart

#### Wishlist
- `POST /v1/user/wishlist/add` - Add to wishlist
- `GET /v1/user/wishlist` - View wishlist
- `DELETE /v1/user/wishlist/remove` - Remove from wishlist

#### Orders
- `GET /v1/user/checkout` - Get checkout summary
- `POST /v1/user/checkout` - Place order
- `GET /v1/user/orders` - List orders
- `GET /v1/user/orders/:id` - Order details
- `POST /v1/user/orders/:id/cancel` - Cancel order
- `POST /v1/user/orders/:id/items/:item_id/cancel` - Cancel specific item
- `POST /v1/user/orders/:id/return` - Return order
- `GET /v1/user/orders/:id/invoice` - Download invoice

#### Payment
- `POST /v1/user/checkout/payment/initiate` - Initiate payment
- `POST /v1/user/checkout/payment/verify` - Verify payment
- `GET /v1/user/checkout/payment/methods` - List payment methods

#### Wallet
- `GET /v1/user/wallet` - Get wallet balance
- `GET /v1/user/wallet/transactions` - List transactions
- `POST /v1/user/wallet/topup/initiate` - Initiate wallet top-up
- `POST /v1/user/wallet/topup/verify` - Verify top-up transaction

#### Coupons
- `GET /v1/user/coupons` - List available coupons
- `POST /v1/user/coupons/apply` - Apply coupon
- `POST /v1/user/coupons/remove` - Remove coupon

### 👨‍💼 Admin Endpoints

#### Authentication & Dashboard
- `POST /v1/admin/login` - Admin login
- `POST /v1/admin/logout` - Admin logout
- `GET /v1/admin/dashboard` - Dashboard overview

#### User Management
- `GET /v1/admin/users` - List all users
- `PUT /v1/admin/users/:id/block` - Block/unblock user

#### Product Management
- `POST /v1/admin/books` - Create book
- `PUT /v1/admin/books/:id` - Update book
- `DELETE /v1/admin/books/:id` - Delete book
- `POST /v1/admin/books/:id/images` - Upload book images
- `PUT /v1/admin/books/field/:field/:value` - Update specific field

#### Category & Genre Management
- `POST /v1/admin/categories` - Create category
- `PUT /v1/admin/categories/:id` - Update category
- `DELETE /v1/admin/categories/:id` - Delete category
- `POST /v1/admin/genres` - Create genre
- `PUT /v1/admin/genres/:id` - Update genre
- `DELETE /v1/admin/genres/:id` - Delete genre

#### Order Management
- `GET /v1/admin/orders` - List all orders
- `GET /v1/admin/orders/:id` - Order details
- `PUT /v1/admin/orders/:id/status` - Update order status
- `GET /v1/admin/sales/report` - Generate sales report
- `POST /v1/admin/orders/:id/return/accept` - Accept return request
- `POST /v1/admin/orders/:id/return/reject` - Reject return request

#### Offer Management
- `POST /v1/admin/offers/products` - Create product offer
- `PUT /v1/admin/offers/products/:id` - Update product offer
- `DELETE /v1/admin/offers/products/:id` - Delete product offer
- `POST /v1/admin/offers/categories` - Create category offer
- `PUT /v1/admin/offers/categories/:id` - Update category offer
- `DELETE /v1/admin/offers/categories/:id` - Delete category offer

---

## Features

### Admin Side
- Secure admin authentication
- User management (block/unblock)
- Category, genre, and product management (CRUD, image upload)
- Inventory/stock management
- Order management (status, returns, search, pagination)
- Reviews moderation

### User Side
- User authentication (signup, login, OTP)
- Product browsing, search, filtering, sorting
- Cart and wishlist management
- Checkout and order placement (COD supported)
- Order tracking, cancellation, return, invoice download
- User profile and address management
- Reviews and ratings

---

## API Endpoints

### Auth
- `GET /auth/google/login` - Google OAuth login
- `GET /auth/google/callback` - Google OAuth callback

### User (Public)
- `POST /v1/register` - User registration
- `POST /v1/login` - User login
- `POST /v1/verify-otp` - Verify OTP
- `POST /v1/forgot-password` - Forgot password
- `POST /v1/verify-reset-otp` - Verify reset OTP
- `POST /v1/reset-password` - Reset password
- `GET /v1/books` - List all books
- `GET /v1/books/:id` - Book details
- `GET /v1/categories` - List categories
- `GET /v1/categories/:id/books` - Books by category

### User (Protected, `/v1/user/` prefix)
- `POST /cart/add` - Add to cart
- `GET /cart` - View cart
- `PUT /cart/update` - Update cart
- `DELETE /cart/remove` - Remove from cart
- `DELETE /cart/clear` - Clear cart
- `POST /wishlist/add` - Add to wishlist
- `GET /wishlist` - View wishlist
- `DELETE /wishlist/remove` - Remove from wishlist
- `GET /checkout` - Get checkout summary
- `POST /checkout` - Place order
- `GET /orders` - List orders
- `GET /orders/:id` - Order details
- `POST /orders/:id/cancel` - Cancel order
- `POST /orders/:id/items/:item_id/cancel` - Cancel order item
- `POST /orders/:id/return` - Return order
- `GET /orders/:id/invoice` - Download invoice
- `POST /books/:id/review` - Add review
- `GET /books/:id/reviews` - Get reviews

### Admin (Protected, `/v1/admin/` prefix)
- `POST /login` - Admin login
- `POST /logout` - Admin logout
- `GET /dashboard` - Dashboard overview
- `GET /users` - List users
- `PUT /users/:id/block` - Block user
- `GET /categories` - List categories
- `POST /categories` - Create category
- `PUT /categories/:id` - Update category
- `DELETE /categories/:id` - Delete category
- `GET /categories/:id/books` - Books by category
- `GET /books` - List books
- `POST /books` - Create book
- `PUT /books/field/:field/:value` - Update book by field
- `GET /books/:id` - Book details
- `PUT /books/:id` - Update book
- `DELETE /books/:id` - Delete book
- `GET /books/:id/check` - Check book exists
- `GET /books/:id/reviews` - Get book reviews
- `PUT /books/:id/reviews/:reviewId/approve` - Approve review
- `DELETE /books/:id/reviews/:reviewId` - Delete review
- `POST /genres` - Create genre
- `PUT /genres/:id` - Update genre
- `DELETE /genres/:id` - Delete genre
- `GET /genres` - List genres
- `GET /genres/:id` - Books by genre
- `GET /orders` - List orders
- `GET /orders/:id` - Order details
- `PUT /orders/:id/status` - Update order status
- `POST /orders/:id/return/accept` - Accept return
- `POST /orders/:id/return/reject` - Reject return

### Profile (Protected, `/v1/profile/` prefix)
- `GET ""` - Get user profile
- `PUT ""` - Update profile
- `PUT /email` - Update email
- `POST /email/verify` - Verify email update
- `PUT /password` - Change password
- `POST /image` - Upload profile image
- `POST /address` - Add address
- `PUT /address/:id` - Edit address
- `DELETE /address/:id` - Delete address
- `PUT /address/:id/default` - Set default address
- `GET /address` - List addresses

---

## Controllers
- `address_controller.go` - Address CRUD for users
- `admin_controller.go` - Admin authentication, dashboard
- `admin_inventory_controller.go` - Admin inventory/stock
- `admin_order_controller.go` - Admin order management
- `auth_controller.go` - User authentication
- `book_controller.go` - Book CRUD, details, search
- `book_controller_by_field.go` - Book update by field
- `cart_controller.go` - Cart operations
- `category_controller.go` - Category CRUD
- `checkout_controller.go` - Checkout summary & order placement
- `genre_controller.go` - Genre CRUD
- `order_controller.go` - User order management
- `user_controller.go` - User account management
- `user_profile_controller.go` - User profile, password, image
- `wishlist_controller.go` - Wishlist operations

---

## Models
- `address.go` - Address model
- `models.go` - User, Book, Category, Genre, etc.
- `order.go` - Order, OrderItem models
- `wishlist.go` - Wishlist model
- `password_history.go` - Password history for security

---

## Setup & Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Govind-619/ReadSphere.git
   cd ReadSphereMVC
   ```
2. **Install dependencies:**
   ```bash
   go mod download
   ```
3. **Configure environment:**
   - Copy `.env.example` to `.env` and fill in DB credentials, admin email/password, JWT secret, etc.
4. **Create the database:**
   ```bash
   createdb readsphere
   ```
5. **Run the application:**
   ```bash
   go run main.go
   ```

---

## Testing

- Use Postman or similar tools to test API endpoints.
- [Postman Collection Link]

---

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

This project is licensed under the MIT License - see the LICENSE file for details.

---

## Recent Upgrades (as of May 2025)

- **Checkout Flow:**
  - `/checkout` endpoints for summary and order placement (Cash on Delivery supported).
  - Full validation, subtotal/discount/tax/final total calculations, and stock management.
- **Order Models:**
  - `Order` and `OrderItem` models for robust order tracking.
- **Cart/Wishlist Integration:**
  - Unified logic for cart and wishlist, consistent user experience.
- **Stock Management:**
  - Automatic inventory adjustment on order/return/cancel.
- **Wishlist Management:**
  - Users can add, view, and remove books from their wishlist. Wishlist automatically syncs with cart actions.
- **Admin Sales Report Export:**
  - Admin can export sales reports as Excel or PDF, including analytics (top/bottom 5 products, top customers, financials, customer insights, order status summary).
- **Offer Management:**
  - Admin can create, list, update, and delete product and category offers. Deleting a non-existent offer returns a RESTful 404 error with an error message.
- **Improved Error Handling:**
  - All offer deletion endpoints now return 404 with descriptive error messages if the offer does not exist, improving RESTful API correctness and user feedback.

---

## Features


### Admin Side
- Secure admin authentication
- User management (block/unblock users)
- Category management (CRUD operations)
- Product management (CRUD operations with image uploads)
- Search and pagination functionality
- **Offer Management**
  - Create, list, update, and delete product offers (with discount percent, date range, active status)
  - Create, list, update, and delete category offers (with discount percent, date range, active status)
  - RESTful 404 error handling for non-existent offer deletion
- **Sales Report Export**
  - Export sales reports as Excel or PDF with analytics (top/bottom products, top customers, financials, order status)
- **Inventory/Stock Management**
  - Manage product stock levels and update based on order/return/cancellation
- **Order Management**
  - List orders in descending order of date
  - View order details (ID, date, user info)
  - Change order status (pending, shipped, out for delivery, delivered, cancelled)
  - Verify and process return requests
  - Search, sort, filter, and paginate orders
- **Review Moderation**
  - Approve or delete reviews on books/products


### User Side
- User authentication (signup, login, OTP verification)
- Product browsing with advanced filtering and sorting
- Product details with reviews and ratings
- Shopping cart functionality
- **Wishlist Management**
  - Add, view, and remove books from wishlist
  - Wishlist automatically syncs with cart (removes from wishlist if added to cart)
- **Checkout and Order Placement**
  - Place orders (Cash on Delivery supported)
  - Checkout summary with address selection, itemized totals, discounts, and taxes
- **Order Tracking & Management**
  - Track orders, view order details, cancel/return orders, download invoices
- **User Profile & Address**
  - Manage profile, change password, upload profile image
  - Add, edit, delete, and set default addresses
- **Cart Management**
  - Add to cart, view cart, update quantities, remove items
  - Prevent adding blocked/unlisted or out-of-stock products
  - Quantity validation based on stock and max limits
- **Robust Error Handling**
  - Clear feedback for invalid actions (e.g., trying to delete a non-existent offer)

  - Prevent adding blocked/unlisted products.
  - If already in cart, increase quantity.
  - Remove product from wishlist if added to cart.
  - Quantity increment/decrement with validation based on stock.
  - Enforce max quantity limit per product.
  - Disable and prevent checkout of out-of-stock items.
- **Checkout Page**
  - Show user addresses with default selection.
  - Add/edit address option available.
  - Display checkout items with:
    - Product image
    - Quantity and item total
    - Optional taxes
    - Discounts
    - Final price summary
- **Order Placement**
  - Cash on Delivery support.
  - Order success page with thank you note and navigation to:
    - Order detail page
    - Continue shopping page
- **Order Management**
  - View order listing with unique Order ID.
  - Cancel entire order or specific products (with optional reason).
    - Cancelled products restock inventory.
  - Return option for delivered orders (reason mandatory).
  - Order detail view.
  - Download order invoice as PDF.
  - Search functionality for orders.

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

## 🚀 Getting Started

### Prerequisites
- Go 1.24.0 or higher
- PostgreSQL 12+
- Make (optional, for using Makefile commands)
- Git

### Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/ReadSphere.git
cd ReadSphere
```

2. Create and configure your .env file:
```bash
# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_user
DB_PASSWORD=your_password
DB_NAME=readsphere
DB_SSL_MODE=disable

# Server Configuration
PORT=8080
GIN_MODE=debug  # Use 'release' in production

# Security
JWT_SECRET=your_secure_jwt_secret
SESSION_SECRET=your_secure_session_key

# OAuth2 (Google)
GOOGLE_CLIENT_ID=your_google_client_id
GOOGLE_CLIENT_SECRET=your_google_client_secret
GOOGLE_CALLBACK_URL=http://localhost:8080/auth/google/callback

# Razorpay
RAZORPAY_KEY_ID=your_razorpay_key
RAZORPAY_SECRET=your_razorpay_secret

# SMTP Configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=your_email@gmail.com
SMTP_PASSWORD=your_app_specific_password
SMTP_FROM_NAME=ReadSphere

# File Upload
UPLOAD_DIR=./uploads
MAX_UPLOAD_SIZE=5242880  # 5MB in bytes
```

3. Initialize the database:
```bash
make migrate
# Or manually: go run scripts/migrate.go
```

4. Start the server:
```bash
make run
# Or manually: go run main.go
```

### Available Make Commands

The project includes several helpful Make commands for common tasks:

```bash
# Start the application
make run

# Build the binary
make build

# Run all tests
make test

# Clean build artifacts
make clean

# Run database migrations
make migrate

# Install project dependencies
make deps

# Format all Go files
make fmt
```

Each command can also be run manually without Make if needed. See the command details in the installation section above.

### Important Security Notes

1. Directory Management:
   - The `uploads/` directory is used for storing book images and other uploads
   - The `logs/` directory is automatically created for application logs
   - Both directories should be properly configured in production for permissions and backup

2. Production Configuration:
   - Set `GIN_MODE=release`
   - Enable HTTPS and update session security settings
   - Use strong, unique secrets for JWT and sessions
   - Configure proper CORS settings
   - Set up rate limiting and DoS protection

3. Sensitive Data:
   - Never commit .env file or any sensitive credentials
   - Use secure credential management in production
   - Regularly rotate secrets and API keys

4. Logging:
   - Logs are rotated automatically to prevent disk space issues
   - Contains four log levels: INFO, WARNING, ERROR, and DEBUG
   - Sensitive data is automatically scrubbed from logs