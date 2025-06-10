.PHONY: run build test clean migrate analyze-logs

# Build the application
build:
	go build -o readsphere main.go

# Run the application
run:
	go run main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f readsphere
	go clean

# Run database migrations
migrate:
	go run main.go migrate

# Install dependencies
deps:
	go mod download

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Create uploads directory
setup:
	mkdir -p uploads

# Create database
db-create:
	createdb readsphere

# Drop database
db-drop:
	dropdb readsphere

# Reset database (drop and recreate)
db-reset: db-drop db-create migrate

# Analyze logs
analyze-logs:
	@echo "Analyzing logs..."
	@go run scripts/analyze_logs.go

# Help command
help:
	@echo "Available commands:"
	@echo "  make build      - Build the application"
	@echo "  make run        - Run the application"
	@echo "  make test       - Run tests"
	@echo "  make clean      - Clean build artifacts"
	@echo "  make migrate    - Run database migrations"
	@echo "  make deps       - Install dependencies"
	@echo "  make fmt        - Format code"
	@echo "  make lint       - Lint code"
	@echo "  make setup      - Create uploads directory"
	@echo "  make db-create  - Create database"
	@echo "  make db-drop    - Drop database"
	@echo "  make db-reset   - Reset database"
	@echo "  make analyze-logs - Analyze logs"
	@echo "  make help       - Show this help message" 