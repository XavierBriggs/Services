.PHONY: build run test test-unit test-integration clean docker-build docker-push

# Binary name
BINARY_NAME=edge-detector
DOCKER_IMAGE=fortuna/edge-detector

# Go commands
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) cmd/edge-detector/main.go

# Run the service locally
run:
	$(GOCMD) run cmd/edge-detector/main.go

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	$(GOTEST) -v -race -timeout 30s ./internal/... ./pkg/... ./sports/...

# Run integration tests (requires Docker)
test-integration:
	$(GOTEST) -v -race -timeout 60s ./tests/integration/...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Build Docker image
docker-build:
	docker build -t $(DOCKER_IMAGE):latest .

# Push Docker image
docker-push:
	docker push $(DOCKER_IMAGE):latest

# Lint code
lint:
	golangci-lint run ./...

# Format code
fmt:
	$(GOCMD) fmt ./...

# Vet code
vet:
	$(GOCMD) vet ./...

