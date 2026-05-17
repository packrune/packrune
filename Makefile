# Packrune — top-level Makefile.
#
# Run `make help` to see the available targets. Every target is one short
# command; we deliberately keep the build system boring.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

GO       ?= go
GOFLAGS  ?=
PKG      := ./...
BIN_DIR  := bin
BIN      := $(BIN_DIR)/packrune

WEB_DIR  := web

VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS  := -s -w \
            -X main.version=$(VERSION) \
            -X main.commit=$(COMMIT) \
            -X main.date=$(DATE)

.PHONY: help
help: ## Show this help.
	@awk 'BEGIN {FS = ":.*##"; printf "Packrune build targets\n\nUsage: make <target>\n\nTargets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the packrune binary.
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/packrune

.PHONY: run
run: ## Build and run with default config.
	$(GO) run $(GOFLAGS) ./cmd/packrune

.PHONY: test
test: ## Run unit tests.
	$(GO) test $(GOFLAGS) -race -count=1 $(PKG)

.PHONY: test-integration
test-integration: ## Run integration tests (slow).
	$(GO) test $(GOFLAGS) -race -count=1 -tags=integration ./test/...

.PHONY: cover
cover: ## Run tests with coverage output.
	$(GO) test $(GOFLAGS) -race -count=1 -coverprofile=coverage.out $(PKG)
	$(GO) tool cover -func=coverage.out | tail -1

.PHONY: lint
lint: ## Run golangci-lint.
	golangci-lint run $(PKG)

.PHONY: fmt
fmt: ## Format Go and frontend source.
	$(GO) fmt $(PKG)
	@if [ -d $(WEB_DIR) ] && [ -f $(WEB_DIR)/package.json ]; then \
		cd $(WEB_DIR) && pnpm run --if-present format; \
	fi

.PHONY: vet
vet: ## Run go vet.
	$(GO) vet $(PKG)

.PHONY: tidy
tidy: ## go mod tidy.
	$(GO) mod tidy

.PHONY: check-headers
check-headers: ## Verify SPDX license headers in source files.
	@./scripts/check-headers.sh

.PHONY: web-install
web-install: ## Install frontend dependencies.
	cd $(WEB_DIR) && pnpm install

.PHONY: web-dev
web-dev: ## Start the frontend dev server.
	cd $(WEB_DIR) && pnpm dev

.PHONY: web-build
web-build: ## Build the frontend for embedding into the binary.
	cd $(WEB_DIR) && pnpm build

.PHONY: clean
clean: ## Remove build artifacts.
	rm -rf $(BIN_DIR) coverage.out coverage.html $(WEB_DIR)/dist
