.PHONY: help build test test-unit test-integration test-e2e clean run init-db docker-build docker-up docker-down lint fmt

# Variables
APP_NAME=app-deployer
BUILD_DIR=bin
MAIN_FILE=cmd/server/main.go
INIT_DB_FILE=scripts/setup/init-db.go

# Help target
help:
	@echo "Available targets:"
	@echo "  build            - Build the application"
	@echo "  test             - Run all unit tests"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests (requires Docker)"
	@echo "  test-e2e         - Run E2E tests (requires GCP credentials)"
	@echo "  clean            - Clean build artifacts"
	@echo "  run              - Run the application"
	@echo "  init-db          - Initialize database"
	@echo "  docker-build     - Build Docker image"
	@echo "  docker-up        - Start Docker services"
	@echo "  docker-down      - Stop Docker services"
	@echo "  lint             - Run linters"
	@echo "  fmt              - Format code"

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(APP_NAME) $(MAIN_FILE)
	@echo "Build complete: $(BUILD_DIR)/$(APP_NAME)"

# Run tests (unit tests only, excludes integration and e2e)
test:
	@echo "Running unit tests..."
	go test -v -coverprofile=coverage.out ./internal/... ./pkg/...
	@echo "Tests complete"

# Alias for unit tests
test-unit: test

# Run integration tests (requires Docker, optionally kind cluster)
test-integration:
	@echo "Running integration tests..."
	go test -v -tags=integration ./tests/integration/...
	@echo "Integration tests complete"

# Run integration tests in short mode (faster, skips slow tests)
test-integration-short:
	@echo "Running integration tests (short mode)..."
	go test -v -short -tags=integration ./tests/integration/...
	@echo "Integration tests complete"

# Run E2E tests (requires GCP credentials and infrastructure)
test-e2e:
	@echo "Running E2E tests..."
	go test -v -tags=e2e ./tests/e2e/...
	@echo "E2E tests complete"

# Run all tests (unit + integration)
test-all: test test-integration
	@echo "All tests complete"

# Run tests with race detection (requires CGO)
test-race:
	@echo "Running tests with race detection..."
	go test -v -race -coverprofile=coverage.out ./internal/... ./pkg/...
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
