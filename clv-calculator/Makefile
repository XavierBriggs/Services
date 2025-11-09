.PHONY: build run test clean docker-build

# Build the service
build:
	go build -o bin/clv-calculator ./cmd/clv-calculator

# Run the service locally
run:
	go run ./cmd/clv-calculator/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker build -t clv-calculator:latest .

