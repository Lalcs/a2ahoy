APP_NAME := a2ahoy
BUILD_DIR := build
GO := go

# Detect current install location via `which`; fall back to $(HOME)/.local/bin
INSTALL_DIR ?= $(shell p=$$(which $(APP_NAME) 2>/dev/null); if [ -n "$$p" ]; then dirname "$$p"; else echo "$(HOME)/.local/bin"; fi)
VERSION := $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/Lalcs/a2ahoy/internal/version.Version=$(VERSION)

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

.PHONY: install
install: ## Build with version info and install to $(INSTALL_DIR), replacing any existing binary
	@mkdir -p $(BUILD_DIR)
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(APP_NAME) .
	@mkdir -p $(INSTALL_DIR)
	install -m 0755 $(BUILD_DIR)/$(APP_NAME) $(INSTALL_DIR)/$(APP_NAME)
	@echo "Installed $(APP_NAME) $(VERSION) -> $(INSTALL_DIR)/$(APP_NAME)"

.PHONY: run
run: build ## Build and show help
	./$(BUILD_DIR)/$(APP_NAME) --help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
