# 🚀 API Endpoints

## 🔓 Public Endpoints

### Authentication
- `GET /auth/google/login` - Google OAuth login
- `GET /auth/google/callback` - Google OAuth callback
- `POST /v1/register` - User registration
- `POST /v1/login` - User login
- `POST /v1/verify-otp` - OTP verification
- `POST /v1/forgot-password` - Password reset request
- `POST /v1/verify-reset-otp` - Verify reset OTP
- `POST /v1/reset-password` - Reset password

### Books & Categories
- `GET /v1/books` - List all books with search, pagination, and filtering
- `GET /v1/books/:id` - Get book details
- `GET /v1/books/:id/images` - Get book images
- `GET /v1/categories` - List categories
- `GET /v1/categories/:id/books` - Books by category
- `GET /v1/genres` - List genres
- `GET /v1/genres/:id/books` - Books by genre

### Referral System
- `GET /v1/referral/:token` - Get referral information
- `GET /v1/referral/invite/:token` - Accept referral invitation

## 🔒 Protected User Endpoints

### Profile Management
- `GET /v1/profile` - Get user profile
- `PUT /v1/profile` - Update basic profile
- `PUT /v1/profile/email` - Update email
- `POST /v1/profile/email/verify` - Verify email update
- `PUT /v1/profile/password` - Change password
- `POST /v1/profile/image` - Upload profile image

### Address Management
- `GET /v1/profile/address` - List addresses
- `POST /v1/profile/address` - Add address
- `PUT /v1/profile/address/:id` - Edit address
- `DELETE /v1/profile/address/:id` - Delete address
- `PUT /v1/profile/address/:id/default` - Set default address

### Shopping Cart
- `POST /v1/user/cart/add` - Add to cart
- `GET /v1/user/cart` - View cart
- `PUT /v1/user/cart/update` - Update quantities
- `DELETE /v1/user/cart/remove` - Remove item
- `DELETE /v1/user/cart/clear` - Clear cart

### Wishlist
- `POST /v1/user/wishlist/add` - Add to wishlist
- `GET /v1/user/wishlist` - View wishlist
- `DELETE /v1/user/wishlist/remove` - Remove from wishlist

### Orders
- `GET /v1/user/checkout` - Get checkout summary
- `POST /v1/user/checkout` - Place order
- `GET /v1/user/orders` - List orders
- `GET /v1/user/orders/:id` - Order details
- `POST /v1/user/orders/:id/cancel` - Cancel order
- `POST /v1/user/orders/:id/items/:item_id/cancel` - Cancel specific item
- `POST /v1/user/orders/:id/return` - Return order
- `GET /v1/user/orders/:id/invoice` - Download invoice

### Payment
- `POST /v1/user/checkout/payment/initiate` - Initiate payment
- `POST /v1/user/checkout/payment/verify` - Verify payment
- `GET /v1/user/checkout/payment/methods` - List payment methods

### Wallet
- `GET /v1/user/wallet` - Get wallet balance
- `GET /v1/user/wallet/transactions` - List transactions
- `POST /v1/user/wallet/topup/initiate` - Initiate wallet top-up
- `POST /v1/user/wallet/topup/verify` - Verify top-up transaction

### Coupons
- `GET /v1/user/coupons` - List available coupons
- `POST /v1/user/coupons/apply` - Apply coupon
- `POST /v1/user/coupons/remove` - Remove coupon

### Referral
- `GET /v1/user/referral/code` - Get user's referral code
- `POST /v1/user/referral/generate` - Generate new referral code

## 👨‍💼 Admin Endpoints

### Authentication & Dashboard
- `POST /v1/admin/login` - Admin login
- `POST /v1/admin/logout` - Admin logout
- `GET /v1/admin/dashboard` - Dashboard overview

### User Management
- `GET /v1/admin/users` - List all users with search and pagination
- `PUT /v1/admin/users/:id/block` - Block/unblock user

### Product Management
- `POST /v1/admin/books` - Create book
- `PUT /v1/admin/books/:id` - Update book
- `DELETE /v1/admin/books/:id` - Delete book
- `POST /v1/admin/books/:id/images` - Upload book images
- `PUT /v1/admin/books/field/:field/:value` - Update specific field

### Category & Genre Management
- `POST /v1/admin/categories` - Create category
- `PUT /v1/admin/categories/:id` - Update category
- `DELETE /v1/admin/categories/:id` - Delete category
- `POST /v1/admin/genres` - Create genre
- `PUT /v1/admin/genres/:id` - Update genre
- `DELETE /v1/admin/genres/:id` - Delete genre

### Order Management
- `GET /v1/admin/orders` - List all orders with search and pagination
- `GET /v1/admin/orders/:id` - Order details
- `PUT /v1/admin/orders/:id/status` - Update order status
- `GET /v1/admin/sales/report` - Generate sales report
- `POST /v1/admin/orders/:id/return/accept` - Accept return request
- `POST /v1/admin/orders/:id/return/reject` - Reject return request
- `GET /v1/admin/sales/report/excel` - Download sales report as Excel
- `GET /v1/admin/sales/report/pdf` - Download sales report as PDF

### Offer Management
- `POST /v1/admin/offers/products` - Create product offer
- `PUT /v1/admin/offers/products/:id` - Update product offer
- `DELETE /v1/admin/offers/products/:id` - Delete product offer
- `POST /v1/admin/offers/categories` - Create category offer
- `PUT /v1/admin/offers/categories/:id` - Update category offer
- `DELETE /v1/admin/offers/categories/:id` - Delete category offer

### Coupon Management
- `POST /v1/admin/coupons` - Create coupon
- `PUT /v1/admin/coupons/:id` - Update coupon
- `DELETE /v1/admin/coupons/:id` - Delete coupon
- `GET /v1/admin/coupons` - List all coupons

### Referral Management
- `GET /v1/admin/referrals` - List all referrals
- `GET /v1/admin/referrals/analytics` - Referral analytics

### Wallet Management
- `GET /v1/admin/wallet/transactions` - List all wallet transactions
- `PUT /v1/admin/wallet/transactions/:id/approve` - Approve wallet transaction

### Delivery Management
- `POST /v1/admin/delivery/charges` - Set delivery charges
- `GET /v1/admin/delivery/charges` - Get delivery charges 