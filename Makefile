# AhaSend CLI Makefile

# Version information. An empty VERSION, such as one from a CI step that
# set nothing, counts as unset.
override VERSION := $(or $(VERSION),dev)
# The commit and its time, as release builds stamp them; unknown outside a
# git checkout
GIT_COMMIT = $(or $(shell scripts/build-info.sh 2>/dev/null | sed -n 's/^GIT_COMMIT=//p'),unknown)
BUILD_TIME = $(or $(shell scripts/build-info.sh 2>/dev/null | sed -n 's/^BUILD_TIME=//p'),unknown)

# The golangci-lint version setup installs; .github/workflows/ci.yml runs the
# same minor version.
GOLANGCI_LINT_VERSION := v2.14.0

# The Go release that builds and tests the CLI: the toolchain line of go.mod.
# The Dockerfile and the workflows name the same release; check-versions
# checks that they do.
GO_TOOLCHAIN := $(shell sed -n 's/^toolchain go//p' go.mod)

# The GoReleaser version that builds the release assets, here and in the
# workflows; check-versions checks that they name it
GORELEASER_VERSION := v2.18.3
GORELEASER := GOTOOLCHAIN=go$(GO_TOOLCHAIN) go run github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

# Build flags
LDFLAGS = -ldflags "-X github.com/AhaSend/ahasend-cli/internal/version.Version=$(VERSION) \
	-X github.com/AhaSend/ahasend-cli/internal/version.BuildTime=$(BUILD_TIME) \
	-X github.com/AhaSend/ahasend-cli/internal/version.GitCommit=$(GIT_COMMIT)"

# Without cgo the binary is static, so it runs on any system of its platform
# whatever C library it has; -trimpath leaves the build machine's paths out.
GO_BUILD = CGO_ENABLED=0 go build -trimpath $(LDFLAGS)

.PHONY: help build release-snapshot test test-unit test-integration test-coverage test-scripts clean fmt lint deps deps-check notices notices-check check-versions

# Default target
help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Build targets
build: ## Build the CLI binary
	@echo "Building ahasend..."
	$(GO_BUILD) -o bin/ahasend .

# A snapshot does not check that the work tree is clean, as a release does,
# so this target checks it: an untracked file would stop the release and
# change the binary's vcs.modified flag. It fails in CI and warns elsewhere.
release-snapshot: ## Build the release assets for every platform into dist/
	@if [ -n "$$(git status --porcelain)" ]; then \
		git status --short >&2; \
		if [ -n "$${CI:-}" ]; then \
			echo "The work tree has changes or untracked files, which a release refuses" >&2; exit 1; \
		fi; \
		echo "Warning: the work tree has changes or untracked files, which a release refuses" >&2; \
	fi
	GIT_COMMIT=$(GIT_COMMIT) BUILD_TIME=$(BUILD_TIME) $(GORELEASER) release --snapshot --clean

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

test-scripts: ## Run the tests of the release scripts
	sh test/release/run.sh

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

# A workflow that sets up Go sets GO_VERSION to the toolchain and passes it to
# every go-version; a workflow that runs GoReleaser sets GORELEASER_VERSION and
# passes it to the version input of every goreleaser-action step, which this
# awk program checks.
GORELEASER_ACTION_CHECK := \
	/uses: goreleaser\/goreleaser-action/ { step = 1; next } \
	step && /^ *- / { exit 1 } \
	step && /^ *version:/ { if ($$0 !~ /version: \$$\{\{ env\.GORELEASER_VERSION \}\}$$/) exit 1; step = 0 } \
	END { if (step) exit 1 }

check-versions: ## Check that the Dockerfile and workflows use go.mod's toolchain and GORELEASER_VERSION
	@grep -q "golang:$(GO_TOOLCHAIN)-alpine" Dockerfile || \
		{ echo "Dockerfile does not build with Go $(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1; }
	@for workflow in .github/workflows/*.yml; do \
		if grep -q 'actions/setup-go' $$workflow && \
			! grep -q "^ *GO_VERSION: '$(GO_TOOLCHAIN)'$$" $$workflow; then \
			echo "$$workflow does not set GO_VERSION to $(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1; \
		fi; \
		if grep -E '^ *GO_VERSION:' $$workflow | grep -qv "GO_VERSION: '$(GO_TOOLCHAIN)'$$" || \
			grep -E '^ *go-version:' $$workflow | grep -qv 'go-version: $${{ env.GO_VERSION }}$$'; then \
			echo "$$workflow uses a Go other than $(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1; \
		fi; \
		if grep -E '^ *GOTOOLCHAIN:' $$workflow | grep -qv "GOTOOLCHAIN: 'go$(GO_TOOLCHAIN)'$$"; then \
			echo "$$workflow sets a GOTOOLCHAIN other than go$(GO_TOOLCHAIN), the toolchain of go.mod" >&2; exit 1; \
		fi; \
		if grep -q 'goreleaser/goreleaser-action' $$workflow && \
			! grep -q "^ *GORELEASER_VERSION: '$(GORELEASER_VERSION)'$$" $$workflow; then \
			echo "$$workflow does not set GORELEASER_VERSION to $(GORELEASER_VERSION), the version of the Makefile" >&2; exit 1; \
		fi; \
		if ! awk '$(GORELEASER_ACTION_CHECK)' $$workflow; then \
			echo "$$workflow runs goreleaser-action without version: \$${{ env.GORELEASER_VERSION }}" >&2; exit 1; \
		fi; \
		if grep -E '^ *GORELEASER_VERSION:' $$workflow | grep -qv "GORELEASER_VERSION: '$(GORELEASER_VERSION)'$$"; then \
			echo "$$workflow uses a GoReleaser other than $(GORELEASER_VERSION), the version of the Makefile" >&2; exit 1; \
		fi; \
	done

# Clean targets
clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/ dist/ completions/
	rm -f coverage.out coverage.html

# Development setup
setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install golang.org/x/tools/cmd/goimports@latest

# Release preparation
release-prep: clean fmt lint test-coverage release-snapshot ## Prepare for release
	@echo "Release preparation complete"

# Quick development cycle
dev: fmt lint test-unit ## Quick development cycle (format, lint, test)
	@echo "Development cycle complete"
