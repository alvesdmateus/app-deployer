.PHONY: help build test clean run init-db docker-build docker-up docker-down lint fmt

# Variables
APP_NAME=app-deployer
BUILD_DIR=bin
MAIN_FILE=cmd/server/main.go
INIT_DB_FILE=scripts/setup/init-db.go

# Help target
help:
	@echo "Available targets:"
	@echo "  build       - Build the application"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  run         - Run the application"
	@echo "  init-db     - Initialize database"
	@echo "  docker-build- Build Docker image"
	@echo "  docker-up   - Start Docker services"
	@echo "  docker-down - Stop Docker services"
	@echo "  lint        - Run linters"
	@echo "  fmt         - Format code"

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Run tests
test:
	@echo "Running tests..."
	go test -v -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with race detection (requires CGO)
test-race:
	@echo "Running tests with race detection..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with coverage report
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Run the application
run:
	@echo "Running $(APP_NAME)..."
	go run $(MAIN_FILE)

# Initialize database
init-db:
	@echo "Initializing database..."
	go run $(INIT_DB_FILE)

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t $(APP_NAME):latest .

# Start Docker services
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d

# Stop Docker services
docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

# Run linters (requires golangci-lint)
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "Format complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

# Run all checks (fmt, lint, test)
check: fmt lint test
	@echo "All checks passed"
