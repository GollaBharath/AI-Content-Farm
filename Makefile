APP_NAME := ai-content-farm

.PHONY: run build test fmt up down

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

fmt:
	gofmt -w cmd internal

up:
	docker compose up --build

down:
	docker compose down
