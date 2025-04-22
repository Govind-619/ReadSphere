# ReadSphere Backend API

A full-featured backend API for an e-commerce platform built with Go, Gin, PostgreSQL, and GORM.

---

## Table of Contents
- [Recent Upgrades](#recent-upgrades)
- [Project Structure](#project-structure)
- [Features](#features)
- [API Endpoints](#api-endpoints)
- [Controllers](#controllers)
- [Models](#models)
- [Setup & Installation](#setup--installation)
- [Testing](#testing)
- [Contributing](#contributing)
- [License](#license)

---

## Recent Upgrades (as of April 2025)
- **Checkout Flow:**
  - `/checkout` endpoints for summary and order placement (Cash on Delivery supported).
  - Full validation, subtotal/discount/tax/final total calculations, and stock management.
- **Order Models:**
  - `Order` and `OrderItem` models for robust order tracking.
- **Cart/Wishlist Integration:**
  - Unified logic for cart and wishlist, consistent user experience.
- **Stock Management:**
  - Automatic inventory adjustment on order/return/cancel.

---

## Project Structure

```
ReadSphereMVC/
├── config/                # Database and app config
├── controllers/           # All controller logic (admin, user, cart, order, etc.)
├── middleware/            # Authentication and other middleware
├── models/                # Database models (User, Book, Order, Wishlist, etc.)
├── routes/                # Route definitions (admin, user, profile, etc.)
├── uploads/               # Uploaded files (e.g., product images)
├── utils/                 # Utility functions and helpers
├── .env                   # Environment variables
├── go.mod, go.sum         # Go module files
├── main.go                # Application entry point
└── README.md              # Project documentation
```

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

## Recent Upgrades (as of April 2025)

- **Checkout Flow:**
  - Implemented `/checkout` endpoints:
    - `POST /checkout` (Place Order): Validates address and cart, calculates subtotal, discounts, tax, and final total. Creates an order and order items, reduces book stock, and clears the cart. Returns full order details.
    - `GET /checkout` (Get Checkout Summary): Returns cart summary with per-item and total calculations (subtotal, discount, tax, final total).
  - Checkout logic matches cart/wishlist conventions and supports Cash on Delivery.
- **Order Models:**
  - Added `Order` and `OrderItem` models to manage order data and items.
- **Cart/Wishlist Integration:**
  - Cart and wishlist logic unified for consistent user experience.
- **Stock Management:**
  - Placing an order reduces book stock. Cancelling/restocking updates inventory accordingly.

---

## Features


### Admin Side
- Secure admin authentication
- User management (block/unblock users)
- Category management (CRUD operations)
- Product management (CRUD operations with image uploads)
- Search and pagination functionality
- **Order Management**
  - List orders in descending order of date.
  - View details like Order ID, date, user information.
  - Change order status: pending, shipped, out for delivery, delivered, cancelled.
  - Verify return requests.
    - Upon verification, return the amount to the user's wallet.
  - Implement search, sort, filter, clear search.
  - Pagination support.
- **Inventory/Stock Management**
  - Manage product stock levels and update based on order/return.

### User Side
- User authentication (signup, login, OTP verification)
- Product browsing with advanced filtering and sorting
- Product details with reviews and ratings
- Shopping cart functionality
- **User Profile**
  - Show profile details including profile image and address.
  - Edit profile on a separate page (not the view page).
    - Email edits require OTP/token verification.
  - Change password functionality.
- **Address Management**
  - Add, edit, and delete user addresses.
- **Cart Management**
  - Add to cart, view cart, and remove items.
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
- `GET /api/v1/user/checkout` - Get checkout summary
- `POST /api/v1/user/checkout` - Place order (Cash on Delivery supported)

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