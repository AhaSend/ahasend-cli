# AhaSend CLI Makefile

# Version information
VERSION ?= dev
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS := -ldflags "-X github.com/AhaSend/ahasend-cli/internal/version.Version=$(VERSION) \
	-X github.com/AhaSend/ahasend-cli/internal/version.BuildTime=$(BUILD_TIME) \
	-X github.com/AhaSend/ahasend-cli/internal/version.GitCommit=$(GIT_COMMIT)"

.PHONY: help build test test-unit test-integration test-coverage clean fmt lint deps

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the CLI binary
	@echo "Building ahasend..."
	go build $(LDFLAGS) -o bin/ahasend .

build-all: ## Build binaries for all platforms
	@echo "Building for all platforms with version $(VERSION)..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/ahasend-linux-amd64 .
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/ahasend-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/ahasend-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/ahasend-windows-amd64.exe .

# Test targets
test: test-unit test-integration ## Run all tests

test-unit: ## Run unit tests
	@echo "Running unit tests..."
	go test -v -race ./internal/... ./cmd/...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	go test -v -race ./test/integration/...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-coverage-ci: ## Run tests with coverage for CI
	@echo "Running tests with coverage for CI..."
	go test -v -race -coverprofile=coverage.out -covermode=atomic ./...

# Development targets
fmt: ## Format Go code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

lint: ## Run linters
	@echo "Running linters..."
	golangci-lint run

lint-fix: ## Run linters with auto-fix
	@echo "Running linters with auto-fix..."
	golangci-lint run --fix

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

deps-upgrade: ## Upgrade dependencies
	@echo "Upgrading dependencies..."
	go get -u ./...
	go mod tidy

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Development setup
setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Release preparation
release-prep: clean fmt lint test-coverage build-all ## Prepare for release
	@echo "Release preparation complete"

# Quick development cycle
dev: fmt lint test-unit ## Quick development cycle (format, lint, test)
	@echo "Development cycle complete"
