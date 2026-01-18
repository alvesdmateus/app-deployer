.PHONY: help build build-api build-worker build-cli test clean run run-api run-worker docker-build docker-up docker-down lint fmt

# Variables
APP_NAME=app-deployer
BUILD_DIR=bin

# Help target
help:
	@echo "Available targets:"
	@echo "  build       - Build all binaries (api, worker, cli)"
	@echo "  build-api   - Build the API server"
	@echo "  build-worker- Build the orchestrator worker"
	@echo "  build-cli   - Build the CLI tool"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  run         - Run the API server"
	@echo "  run-api     - Run the API server"
	@echo "  run-worker  - Run the orchestrator worker"
	@echo "  docker-build- Build Docker image"
	@echo "  docker-up   - Start Docker services"
	@echo "  docker-down - Stop Docker services"
	@echo "  lint        - Run linters"
	@echo "  fmt         - Format code"

# Build all binaries
build: build-api build-worker build-cli
	@echo "All binaries built successfully"

build-api:
	@echo "Building API server..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/api ./cmd/api
	@echo "Build complete: $(BUILD_DIR)/api"

build-worker:
	@echo "Building orchestrator worker..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/worker ./cmd/worker
	@echo "Build complete: $(BUILD_DIR)/worker"

build-cli:
	@echo "Building CLI tool..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/deployer ./cmd/cli
	@echo "Build complete: $(BUILD_DIR)/deployer"

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

# Run the API server (default)
run: run-api

run-api:
	@echo "Running API server..."
	go run ./cmd/api

run-worker:
	@echo "Running orchestrator worker..."
	go run ./cmd/worker

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
