.PHONY: build run test clean docker-build

# Build the service
build:
	go build -o bin/settlement-service ./cmd/settlement-service

# Run the service locally
run:
	go run ./cmd/settlement-service/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker build -t settlement-service:latest .



