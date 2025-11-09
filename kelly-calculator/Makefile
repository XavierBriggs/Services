.PHONY: build run test clean docker-build

# Build the service
build:
	go build -o bin/kelly-calculator ./cmd/kelly-calculator

# Run the service locally
run:
	go run ./cmd/kelly-calculator/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker build -t kelly-calculator:latest .

# Run in Docker
docker-run:
	docker run -p 8084:8084 \
		-e DEFAULT_BANKROLL=10000 \
		-e KELLY_DEFAULT_FRACTION=0.25 \
		kelly-calculator:latest

