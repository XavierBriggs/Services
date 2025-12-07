.PHONY: build run test clean docker-build docker-run deps

# Binary name
BINARY=slack-gateway

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build the binary
build:
	$(GOBUILD) -o bin/$(BINARY) ./cmd/slack-gateway

# Run the service
run:
	$(GORUN) ./cmd/slack-gateway

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -cover ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker build -t fortuna/slack-gateway:latest .

# Run in Docker
docker-run:
	docker run -p 8095:8095 \
		-e SLACK_SIGNING_SECRET=$(SLACK_SIGNING_SECRET) \
		-e SLACK_BOT_TOKEN=$(SLACK_BOT_TOKEN) \
		-e REDIS_URL=host.docker.internal:6380 \
		-e HOLOCRON_DSN="postgres://fortuna:fortuna_dev_password@host.docker.internal:5436/holocron?sslmode=disable" \
		-e ALEXANDRIA_DSN="postgres://fortuna:fortuna_dev_password@host.docker.internal:5435/alexandria?sslmode=disable" \
		-e BET_API_URL=http://host.docker.internal:8081 \
		fortuna/slack-gateway:latest

# Lint
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .

# Generate go.sum
tidy:
	$(GOMOD) tidy







