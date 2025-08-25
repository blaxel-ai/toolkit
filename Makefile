ARGS:= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

sdk:
	@echo "Downloading controlplane definition from blaxel-ai/controlplane"
	@curl -H "Authorization: token $$(gh auth token)" \
		-H "Accept: application/vnd.github.v3.raw" \
		-o ./definition.yml \
		https://api.github.com/repos/blaxel-ai/controlplane/contents/api/api/definitions/controlplane.yml?ref=main
	oapi-codegen -package=sdk \
		-generate=types,client,spec \
		-o=sdk/blaxel.go \
		-templates=./templates/go \
		definition.yml

# Get git commit hash automatically
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

build:
	@echo "🔨 Building with commit: $(GIT_COMMIT_SHORT)"
	LDFLAGS="-X github.com/blaxel-ai/toolkit/cli/core.commit=$(GIT_COMMIT)" goreleaser release --snapshot --clean --skip homebrew
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		if [ -d "./dist/blaxel_darwin_arm64" ]; then \
			cp ./dist/blaxel_darwin_arm64/blaxel ~/.local/bin/blaxel; \
		else \
			cp ./dist/blaxel_darwin_arm64_v8.0/blaxel ~/.local/bin/blaxel; \
		fi; \
		cp ~/.local/bin/blaxel ~/.local/bin/bl; \
	fi

# Build SDK examples/tests with commit hash
build-sdk:
	@echo "🔨 Building SDK with commit: $(GIT_COMMIT_SHORT)"
	go build -ldflags "-X github.com/blaxel-ai/toolkit/sdk.commitHash=$(GIT_COMMIT)" ./sdk/...

# Development build without goreleaser
build-dev:
	@echo "🔨 Development build with commit: $(GIT_COMMIT_SHORT)"
	go build -ldflags "-X github.com/blaxel-ai/toolkit/cli/core.commit=$(GIT_COMMIT)" -o ./bin/blaxel ./
	@echo "✅ Binary built: ./bin/blaxel"

doc:
	rm -rf docs
	go run main.go docs --format=markdown --output=docs
	rm docs/bl_completion_zsh.md docs/bl_completion_bash.md

lint:
	golangci-lint run

test:
	go test -count=1 ./...

test-integration:
	@echo "🧪 Running CLI integration tests..."
	@if [ -z "$$BL_API_KEY" ]; then \
		echo "❌ Error: BL_API_KEY environment variable is required"; \
		echo "   Please set your API key: export BL_API_KEY=your_api_key"; \
		exit 1; \
	fi
	@if [ -z "$$BL_WORKSPACE" ]; then \
		echo "⚠️  Warning: BL_WORKSPACE not set, using 'main' workspace"; \
		export BL_WORKSPACE=main; \
	fi
	@echo "🔑 Using API key: $${BL_API_KEY:0:8}..."
	@echo "🏢 Using workspace: $${BL_WORKSPACE:-main}"
	@echo "🚀 Starting integration tests (this may take several minutes)..."
	go test -count=1 -v -timeout=30m -run TestCLIWorkflow_CompleteFlow ./test/integration/

install:
	uv pip install openapi-python-client

tag:
	git tag -a v$(ARGS) -m "Release v$(ARGS)"
	git push origin v$(ARGS)

%:
	@:

.PHONY: sdk test test-integration