# 🚀 Getting Started

## Prerequisites
- Go 1.23.0 or higher
- PostgreSQL 12+
- Make (optional, for using Makefile commands)
- Git

## Installation

1. **Clone the repository:**
   ```bash
   git clone https://github.com/Govind-619/ReadSphere.git
   cd ReadSphere
   ```

2. **Create and configure your .env file:**
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

   # Frontend URL (for CORS)
   FRONTEND_URL=http://localhost:3000
   ```

3. **Initialize the database:**
   ```bash
   make migrate
   # Or manually: go run scripts/migrate.go
   ```

4. **Start the server:**
   ```bash
   make run
   # Or manually: go run main.go
   ```

## Available Make Commands

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

Each command can also be run manually without Make if needed.

## Environment Variables

Create a `.env` file in the project root with the following variables:

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=your_db_user
DB_PASSWORD=your_db_password
DB_NAME=readsphere
JWT_SECRET=your_jwt_secret
PORT=8080
ENV=development
RAZORPAY_KEY_ID=your_razorpay_key
RAZORPAY_KEY_SECRET=your_razorpay_secret
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=your_email@example.com
SMTP_PASSWORD=your_email_password
FRONTEND_URL=http://localhost:3000
```

> **Tip:** Never commit your real `.env` file to version control. Use `.env.example` for sharing variable names only.

## Important Security Notes

1. **Directory Management:**
   - The `uploads/` directory is used for storing book images and other uploads
   - The `logs/` directory is automatically created for application logs
   - Both directories should be properly configured in production for permissions and backup

2. **Production Configuration:**
   - Set `GIN_MODE=release`
   - Enable HTTPS and update session security settings
   - Use strong, unique secrets for JWT and sessions
   - Configure proper CORS settings
   - Set up rate limiting and DoS protection

3. **Sensitive Data:**
   - Never commit .env file or any sensitive credentials
   - Use secure credential management in production
   - Regularly rotate secrets and API keys

4. **Logging:**
   - Logs are rotated automatically to prevent disk space issues
   - Contains four log levels: INFO, WARNING, ERROR, and DEBUG
   - Sensitive data is automatically scrubbed from logs

## Deployment

- **Build for production:**
  ```sh
  go build -o readsphere
  ./readsphere
  ```
- **Or use Docker:**
  ```sh
  docker build -t readsphere .
  docker run -p 8080:8080 readsphere
  ```
- **Set all required environment variables in your production environment.**

## Testing

- Use Postman or similar tools to test API endpoints.
- [Postman Collection Link]

## Troubleshooting & FAQ

- **Database connection errors:**
  - Ensure PostgreSQL is running and credentials in `.env` are correct.
- **Missing environment variables:**
  - Double-check your `.env` file and variable names.
- **Port already in use:**
  - Change the `PORT` variable in your `.env` file.
- **Email not sending:**
  - Verify SMTP credentials and network access.
- **Other issues:**
  - Check logs in the `logs/` directory for more details. 