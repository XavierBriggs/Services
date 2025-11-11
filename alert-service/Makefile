.PHONY: build run test test-unit test-integration clean docker-build

BINARY_NAME=alert-service
DOCKER_IMAGE=fortuna/alert-service

build:
	go build -o bin/$(BINARY_NAME) cmd/alert-service/main.go

run:
	go run cmd/alert-service/main.go

test: test-unit test-integration

test-unit:
	go test -v -race -timeout 30s ./internal/... ./pkg/...

test-integration:
	go test -v -race -timeout 60s ./tests/integration/...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	go clean
	rm -f bin/$(BINARY_NAME)
	rm -f coverage.out coverage.html

deps:
	go mod download
	go mod tidy

docker-build:
	docker build -t $(DOCKER_IMAGE):latest .

fmt:
	go fmt ./...

vet:
	go vet ./...


