.PHONY: build run test clean docker-build

# Build the service
build:
	go build -o bin/bot-service ./cmd/bot-service

# Run the service locally
run:
	go run ./cmd/bot-service/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker build -t bot-service:latest .

# Run in Docker
docker-run:
	docker run -p 8090:8090 \
		-e TALOS_BOT_MANAGER_URL=http://localhost:5000 \
		-e HOLOCRON_DSN=postgres://... \
		-e ALEXANDRIA_DSN=postgres://... \
		bot-service:latest

