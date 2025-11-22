SHELL := /bin/bash

.PHONY: tidy test run build docker-up docker-down

tidy:
	cd control-plane && go mod tidy

test:
	cd control-plane && go test ./...

run:
	cd control-plane && go run ./cmd/kokoa-cp

build:
	cd control-plane && go build -o bin/kokoa-cp ./cmd/kokoa-cp

docker-up:
	docker compose up --build

docker-down:
	docker compose down
