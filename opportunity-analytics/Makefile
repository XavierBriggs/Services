# Makefile for Opportunity Analytics Service

.PHONY: all build build-enricher run run-enricher test clean docker-build docker-build-enricher docker-push

# Variables
SERVICE_NAME := opportunity-analytics
ENRICHER_NAME := bet-enricher
BINARY := bin/$(SERVICE_NAME)
ENRICHER_BINARY := bin/$(ENRICHER_NAME)
DOCKER_IMAGE := fortuna/$(SERVICE_NAME)
DOCKER_ENRICHER_IMAGE := fortuna/$(ENRICHER_NAME)
DOCKER_TAG := latest

all: build build-enricher

# Build the main analytics service
build:
	@echo "Building $(SERVICE_NAME)..."
	@mkdir -p bin
	go build -o $(BINARY) ./cmd/$(SERVICE_NAME)
	@echo "✓ Build complete: $(BINARY)"

# Build the bet enricher service
build-enricher:
	@echo "Building $(ENRICHER_NAME)..."
	@mkdir -p bin
	go build -o $(ENRICHER_BINARY) ./cmd/enricher
	@echo "✓ Build complete: $(ENRICHER_BINARY)"

# Run the main service locally
run: build
	@echo "Running $(SERVICE_NAME)..."
	./$(BINARY)

# Run the enricher service locally
run-enricher: build-enricher
	@echo "Running $(ENRICHER_NAME)..."
	./$(ENRICHER_BINARY)

# Run both services (in background)
run-all: build build-enricher
	@echo "Starting both services..."
	./$(BINARY) &
	./$(ENRICHER_BINARY) &
	@echo "✓ Both services started"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

# Build Docker image for main service
docker-build:
	@echo "Building Docker image..."
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built: $(DOCKER_IMAGE):$(DOCKER_TAG)"

# Build Docker image for enricher service
docker-build-enricher:
	@echo "Building Enricher Docker image..."
	docker build -f Dockerfile.enricher -t $(DOCKER_ENRICHER_IMAGE):$(DOCKER_TAG) .
	@echo "✓ Docker image built: $(DOCKER_ENRICHER_IMAGE):$(DOCKER_TAG)"

# Build all Docker images
docker-build-all: docker-build docker-build-enricher
	@echo "✓ All Docker images built"

# Push Docker images
docker-push: docker-build docker-build-enricher
	@echo "Pushing Docker images..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_ENRICHER_IMAGE):$(DOCKER_TAG)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Development: watch and rebuild on changes
dev:
	@echo "Running in development mode..."
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

# Verify all files compile
verify:
	@echo "Verifying compilation..."
	go build ./...
	@echo "✓ All files compile successfully"

# Run enricher with backfill for testing
backfill: build-enricher
	@echo "Running enricher backfill for last 7 days..."
	./$(ENRICHER_BINARY) --backfill --days 7

# Show help
help:
	@echo "Available targets:"
	@echo "  build           - Build main analytics service"
	@echo "  build-enricher  - Build bet enricher service"
	@echo "  run             - Run main analytics service"
	@echo "  run-enricher    - Run bet enricher service"
	@echo "  run-all         - Run both services (background)"
	@echo "  test            - Run tests"
	@echo "  clean           - Clean build artifacts"
	@echo "  docker-build    - Build main service Docker image"
	@echo "  docker-build-enricher - Build enricher Docker image"
	@echo "  docker-build-all - Build all Docker images"
	@echo "  deps            - Download dependencies"
	@echo "  fmt             - Format code"
	@echo "  lint            - Run linter"
	@echo "  dev             - Run in development mode (with hot reload)"
	@echo "  verify          - Verify all files compile"
