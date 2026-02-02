.PHONY: build run test lint docker-build docker-push clean dev tidy

# Variables
BINARY_NAME=sandbox-engine
VERSION?=dev
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildTime=$(BUILD_TIME)"

# Docker
DOCKER_REGISTRY?=ghcr.io/terra-clan
DOCKER_IMAGE=$(DOCKER_REGISTRY)/sandbox-engine
DOCKER_TAG?=$(VERSION)

# Build
build:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/sandbox-engine

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/sandbox-engine

# Run
run: build
	./bin/$(BINARY_NAME)

dev:
	go run ./cmd/sandbox-engine

# Test
test:
	go test -v -race -coverprofile=coverage.out ./...

test-short:
	go test -v -short ./...

coverage: test
	go tool cover -html=coverage.out -o coverage.html

# Lint
lint:
	golangci-lint run ./...

lint-fix:
	golangci-lint run --fix ./...

# Dependencies
tidy:
	go mod tidy

vendor:
	go mod vendor

# Docker
docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .
	docker tag $(DOCKER_IMAGE):$(DOCKER_TAG) $(DOCKER_IMAGE):latest

docker-push:
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_IMAGE):latest

docker-run:
	docker run --rm -it \
		-p 8080:8080 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		--env-file .env \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

# Development services
services-up:
	docker compose -f docker/services/docker-compose.yml up -d

services-down:
	docker compose -f docker/services/docker-compose.yml down

services-logs:
	docker compose -f docker/services/docker-compose.yml logs -f

# Clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the binary"
	@echo "  build-linux  - Build for Linux AMD64"
	@echo "  run          - Build and run"
	@echo "  dev          - Run with go run"
	@echo "  test         - Run tests with coverage"
	@echo "  test-short   - Run short tests"
	@echo "  lint         - Run linter"
	@echo "  lint-fix     - Run linter with auto-fix"
	@echo "  tidy         - Run go mod tidy"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-push  - Push Docker image"
	@echo "  docker-run   - Run in Docker"
	@echo "  services-up  - Start dev services (postgres, redis, traefik)"
	@echo "  services-down- Stop dev services"
	@echo "  clean        - Clean build artifacts"
