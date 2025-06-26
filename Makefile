.PHONY: help lint lint-fix test build clean install-tools

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
BINARY_DIR=bin

# Linting
GOLANGCI_LINT_VERSION=v1.62.0

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

install-tools: ## Install development tools
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	@echo "Tools installed successfully"

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	@golangci-lint run --fix ./...

test: ## Run tests
	@echo "Running tests..."
	@$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html

test-short: ## Run short tests only
	@echo "Running short tests..."
	@$(GOTEST) -short -v ./...

build: ## Build all services
	@echo "Building all services..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -o $(BINARY_DIR)/ ./cmd/...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html

fmt: ## Format code
	@echo "Formatting code..."
	@gofmt -s -w .
	@goimports -w .

mod-tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	@$(GOMOD) tidy

check: lint test ## Run all checks (lint + test)

ci: mod-tidy fmt lint test ## Run CI pipeline locally

.DEFAULT_GOAL := help