.PHONY: build run test test-unit test-integration deps clean

# Build the service
build:
	go build -o bin/ws-broadcaster ./cmd/ws-broadcaster

# Run the service
run:
	go run ./cmd/ws-broadcaster/main.go

# Install dependencies
deps:
	go mod tidy
	go mod download

# Run all tests
test: test-unit test-integration

# Run unit tests
test-unit:
	go test -v -race -cover ./tests/unit/...

# Run integration tests (requires Redis)
test-integration:
	go test -v -race -tags=integration ./tests/integration/...

# Clean build artifacts
clean:
	rm -rf bin/
	go clean

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	golangci-lint run

# Build Docker image
docker-build:
	docker build -t ws-broadcaster:latest .

# Generate test coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out




