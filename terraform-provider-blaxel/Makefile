.PHONY: help build install test clean docs integration-test validate-examples check-env

VERSION ?= dev
BINARY = terraform-provider-blaxel
BUILD_FLAGS = -ldflags "-X main.version=$(VERSION)"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the provider binary
	go build $(BUILD_FLAGS) -o $(BINARY)

install: build ## Install provider to local Terraform plugins directory
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/blaxel-ai/blaxel/$(VERSION)/darwin_arm64
	cp $(BINARY) ~/.terraform.d/plugins/registry.terraform.io/blaxel-ai/blaxel/$(VERSION)/darwin_arm64/

test: ## Run unit tests
	go test ./internal/... -v

check-env: ## Check if required environment variables are set
	@echo "Checking environment variables..."
	@if [ -z "$$BL_API_KEY" ] && [ -z "$$BL_API_KEY" ]; then \
		echo "ERROR: Neither BL_API_KEY nor BL_API_KEY is set"; \
		echo "Run: export BL_API_KEY=your-api-key"; \
		exit 1; \
	fi
	@if [ -z "$$BL_WORKSPACE" ] && [ -z "$$BL_WORKSPACE" ]; then \
		echo "ERROR: Neither BL_WORKSPACE nor BL_WORKSPACE is set"; \
		echo "Run: export BL_WORKSPACE=your-workspace"; \
		exit 1; \
	fi
	@echo "✓ Environment variables are set"

integration-test: build check-env ## Run integration tests (creates real resources)
	@echo "Running integration tests..."
	@echo "Note: This will create and destroy real resources in your Blaxel workspace"
	@sleep 2
	cd test && go test -v -timeout 30m

validate-examples: build ## Validate example configurations without applying
	@echo "Validating examples (no resources created)..."
	cd test && go test -v -run TestValidateExamples

clean: ## Clean build artifacts and test state
	rm -f $(BINARY)
	rm -rf dist/
	cd examples/sandbox && rm -rf .terraform .terraform.lock.hcl terraform.tfstate* || true
	cd examples/sandbox_cluster && rm -rf .terraform .terraform.lock.hcl terraform.tfstate* || true

docs: ## Generate provider documentation
	go generate

fmt: ## Format Go and Terraform files
	go fmt ./...
	terraform fmt -recursive examples/

lint: ## Run linter
	golangci-lint run

.DEFAULT_GOAL := help

