APP_NAME := a2ahoy
BUILD_DIR := build
GO := go

.PHONY: all build test lint fmt vet clean tidy run help

all: fmt vet test build ## Run fmt, vet, test, build

build: ## Build binary to build/
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) .

test: ## Run tests
	$(GO) test ./...

test-v: ## Run tests with verbose output
	$(GO) test -v ./...

test-cover: ## Run tests with coverage
	@mkdir -p $(BUILD_DIR)
	$(GO) test -cover -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html

lint: ## Run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...

fmt: ## Format code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

tidy: ## Tidy modules
	$(GO) mod tidy

clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

run: build ## Build and show help
	./$(BUILD_DIR)/$(APP_NAME) --help

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(Makefile_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
