.PHONY: help dev build test lint clean migrate docker-up docker-down

# Default target
help:
	@echo "app-deployer - Development Commands"
	@echo ""
	@echo "Usage:"
	@echo "  make dev              Start local development environment"
	@echo "  make build            Build all binaries"
	@echo "  make test             Run unit tests"
	@echo "  make test-integration Run integration tests"
	@echo "  make test-e2e         Run end-to-end tests"
	@echo "  make lint             Run linters"
	@echo "  make fmt              Format code"
	@echo "  make migrate          Run database migrations"
	@echo "  make docker-up        Start Docker services"
	@echo "  make docker-down      Stop Docker services"
	@echo "  make clean            Clean build artifacts"
	@echo ""

# Start local development environment
dev: docker-up
	@echo "Local development environment started"
	@echo "PostgreSQL: localhost:5432 (user: deployer, password: deployer_dev_password)"
	@echo "Redis: localhost:6379"
	@echo "PgAdmin: http://localhost:5050 (run 'make tools' to start)"
	@echo ""
	@echo "Run 'make run-api' to start the API server"
	@echo "Run 'make run-worker' to start the orchestrator worker"

# Start optional tools (pgadmin, redis-commander)
tools:
	docker-compose --profile tools up -d pgadmin redis-commander
	@echo "Tools started:"
	@echo "  PgAdmin: http://localhost:5050"
	@echo "  Redis Commander: http://localhost:8081"

# Build all binaries
build:
	@echo "Building binaries..."
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/deployer ./cmd/cli
	@echo "Build complete. Binaries in ./bin/"

# Run API server
run-api:
	@echo "Starting API server..."
	go run cmd/api/main.go

# Run orchestrator worker
run-worker:
	@echo "Starting orchestrator worker..."
	go run cmd/worker/main.go

# Run CLI
run-cli:
	@echo "Starting CLI..."
	go run cmd/cli/main.go

# Run unit tests
test:
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Coverage report saved to coverage.out"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./tests/integration/...

# Run E2E tests
test-e2e:
	@echo "Running E2E tests (requires GCP credentials)..."
	go test -v -tags=e2e ./tests/e2e/...

# View coverage report
coverage:
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin"; \
	fi

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -s -w .
	go mod tidy

# Run database migrations
migrate:
	@echo "Running database migrations..."
	@echo "Migrations will be applied when the application starts"
	@echo "Or implement a separate migration tool here"

# Start Docker services
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d postgres redis
	@echo "Waiting for services to be healthy..."
	@sleep 5
	@docker-compose ps

# Stop Docker services
docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -rf dist/
	rm -f coverage.out coverage.html
	@echo "Clean complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest
	@echo "Tools installed"

# Generate Go files (e.g., swagger docs)
generate:
	@echo "Generating Go files..."
	go generate ./...

# Docker build for production
docker-build:
	@echo "Building Docker images..."
	docker build -t app-deployer-api:latest -f Dockerfile.api .
	docker build -t app-deployer-worker:latest -f Dockerfile.worker .
	@echo "Docker images built"

# Initialize project (run once)
init: deps install-tools docker-up
	@echo "Project initialized successfully!"
	@echo "Run 'make dev' to start development"
