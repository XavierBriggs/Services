.PHONY: build run test clean docker-build

build:
	go build -o bin/game-stats-service ./cmd/game-stats-service

run:
	go run ./cmd/game-stats-service

test:
	go test -v ./...

clean:
	rm -rf bin/

docker-build:
	docker build -t fortuna/game-stats-service .

docker-run:
	docker run --rm \
		-e REDIS_URL=${REDIS_URL} \
		fortuna/game-stats-service


