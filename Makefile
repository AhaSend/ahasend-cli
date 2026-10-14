# AhaSend CLI Makefile

# Version information. An empty VERSION, such as one from a CI step that
# set nothing, counts as unset.
override VERSION := $(or $(VERSION),dev)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# The golangci-lint version setup installs; .github/workflows/ci.yml runs the
# same minor version.
GOLANGCI_LINT_VERSION := v2.14.0

# The Go release that builds and tests the CLI: the toolchain line of go.mod.
# The Dockerfile and the workflows name the same release; check-go-version
# checks that they do.
GO_TOOLCHAIN := $(shell sed -n 's/^toolchain go//p' go.mod)

# Build flags
LDFLAGS := -ldflags "-X github.com/AhaSend/ahasend-cli/internal/version.Version=$(VERSION) \
	-X github.com/AhaSend/ahasend-cli/internal/version.BuildTime=$(BUILD_TIME) \
	-X github.com/AhaSend/ahasend-cli/internal/version.GitCommit=$(GIT_COMMIT)"

# Without cgo the binary is static, so it runs on any system of its platform
# whatever C library it has; -trimpath leaves the build machine's paths out.
GO_BUILD := CGO_ENABLED=0 go build -trimpath $(LDFLAGS)

.PHONY: help build build-all test test-unit test-integration test-coverage clean fmt lint deps deps-check notices notices-check check-go-version

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the CLI binary
	@echo "Building ahasend..."
	$(GO_BUILD) -o bin/ahasend .

build-all: ## Build binaries for all platforms
	@echo "Building for all platforms with version $(VERSION)..."
	GOOS=linux GOARCH=amd64 $(GO_BUILD) -o bin/ahasend-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GO_BUILD) -o bin/ahasend-linux-arm64 .
	GOOS=darwin GOARCH=amd64 $(GO_BUILD) -o bin/ahasend-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GO_BUILD) -o bin/ahasend-darwin-arm64 .
	GOOS=windows GOARCH=amd64 $(GO_BUILD) -o bin/ahasend-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 $(GO_BUILD) -o bin/ahasend-windows-arm64.exe .

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

lint: ## Run linters, on the Windows-only files too
	@echo "Running linters..."
	golangci-lint run
	GOOS=windows golangci-lint run

lint-fix: ## Run linters with auto-fix
	@echo "Running linters with auto-fix..."
	golangci-lint run --fix

# Dependency management
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

deps-check: ## Check that go.mod and go.sum are tidy, without changing them
	go mod tidy -diff

deps-upgrade: ## Upgrade dependencies
	@echo "Upgrading dependencies..."
	go get -u ./...
	go mod tidy

# Licence notices. The script runs with the Go release of go.mod's toolchain
# line, which the go command downloads when it is not the one installed: its
# GOROOT holds the standard library's licence, which a Go a distribution
# packaged may keep elsewhere. When the script fails, its error says why.
NOTICES := GOTOOLCHAIN=go$(GO_TOOLCHAIN) ./scripts/third-party-notices.sh

notices: ## Regenerate THIRD_PARTY_NOTICES from the modules in the binary
	@notices=$$(mktemp) && trap 'rm -f "$$notices"' EXIT && \
		$(NOTICES) > "$$notices" && cp "$$notices" THIRD_PARTY_NOTICES

notices-check: ## Check that THIRD_PARTY_NOTICES is current
	@notices=$$(mktemp) && trap 'rm -f "$$notices"' EXIT && \
		$(NOTICES) > "$$notices" && \
		{ diff -u THIRD_PARTY_NOTICES "$$notices" || \
			{ echo "THIRD_PARTY_NOTICES is out of date; run make notices" >&2; exit 1; }; }

check-go-version: ## Check that the Dockerfile and workflows use go.mod's toolchain
	@grep -q "golang:$(GO_TOOLCHAIN)-alpine" Dockerfile || \
		(echo "Dockerfile does not build with Go $(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1)
	@for workflow in .github/workflows/ci.yml .github/workflows/release.yml; do \
		grep -q "GO_VERSION: '$(GO_TOOLCHAIN)'" $$workflow || \
			{ echo "$$workflow does not use Go $(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1; }; \
	done

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

# Development setup
setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/goimports@latest

# Release preparation
release-prep: clean fmt lint test-coverage build-all ## Prepare for release
	@echo "Release preparation complete"

# Quick development cycle
dev: fmt lint test-unit ## Quick development cycle (format, lint, test)
	@echo "Development cycle complete"
