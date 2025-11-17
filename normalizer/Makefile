.PHONY: help build run test test-unit test-integration test-coverage clean lint fmt

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the normalizer binary
	@echo "Building normalizer..."
	@go build -o bin/normalizer cmd/normalizer/main.go
	@echo "✓ Build complete: bin/normalizer"

run: ## Run the normalizer (requires .env or exported vars)
	@echo "Starting normalizer..."
	@go run cmd/normalizer/main.go

test: ## Run all tests
	@echo "Running all tests..."
	@go test -v -race ./...

test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -v -race ./tests/unit/...

test-integration: ## Run integration tests only (requires Redis)
	@echo "Running integration tests..."
	@go test -v -race -tags=integration ./tests/integration/...

test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

lint: ## Run linters
	@echo "Running linters..."
	@golangci-lint run || echo "Note: Install golangci-lint for linting"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "✓ Code formatted"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/ coverage.out coverage.html
	@echo "✓ Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy
	@echo "✓ Dependencies updated"

# Development targets
dev: ## Run in development mode with hot reload (requires air)
	@air || echo "Install air: go install github.com/cosmtrek/air@latest"

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t fortuna-normalizer:latest .
	@echo "✓ Docker image built"

docker-run: ## Run in Docker
	@docker run --rm --env-file .env fortuna-normalizer:latest





