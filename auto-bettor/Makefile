.PHONY: build run test clean

# Variables
BINARY_NAME=auto-bettor
BUILD_DIR=bin
SRC_DIR=cmd/auto-bettor

# Build the service
build:
	@echo "Building auto-bettor..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./$(SRC_DIR)
	@echo "✓ Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the service locally
run: build
	@echo "Running auto-bettor..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "✓ Clean complete"

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	@echo "✓ Format complete"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	@echo "✓ Dependencies downloaded"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy
	@echo "✓ Dependencies tidied"


