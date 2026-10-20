.PHONY: build test lint run container

build:
	go build -o bin/decision-gateway ./cmd/decision-gateway

test:
	go test ./...

lint:
	go vet ./...

run:
	go run ./cmd/decision-gateway

container:
	docker build -t decision-gateway:dev .
