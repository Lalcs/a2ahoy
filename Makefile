APP_NAME := a2ahoy
BUILD_DIR := build
GO := go

.PHONY: all
all: fmt vet test build ## Run fmt, vet, test, build

.PHONY: build
build: ## Build binary to build/
	@mkdir -p $(BUILD_DIR)
	$(GO) build -o $(BUILD_DIR)/$(APP_NAME) .

.PHONY: test
test: ## Run tests
	$(GO) test ./...

.PHONY: test-v
test-v: ## Run tests with verbose output
	$(GO) test -v ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage
	@mkdir -p $(BUILD_DIR)
	$(GO) test -cover -coverprofile=$(BUILD_DIR)/coverage.out ./...
	$(GO) tool cover -html=$(BUILD_DIR)/coverage.out -o $(BUILD_DIR)/coverage.html

.PHONY: lint
lint: ## Run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
	staticcheck ./...

.PHONY: fmt
fmt: ## Format code
	$(GO) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GO) vet ./...

.PHONY: tidy
tidy: ## Tidy modules
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BUILD_DIR)

.PHONY: run
run: build ## Build and show help
	./$(BUILD_DIR)/$(APP_NAME) --help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
