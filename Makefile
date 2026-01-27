ARGS:= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

# Get git commit hash automatically
GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo "unknown")
GIT_COMMIT_SHORT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

build:
	@echo "🔨 Building with commit: $(GIT_COMMIT_SHORT)"
	goreleaser release --snapshot --clean --skip homebrew
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		if [ -d "./dist/blaxel_darwin_arm64" ]; then \
			cp ./dist/blaxel_darwin_arm64/blaxel ~/.local/bin/blaxel; \
		else \
			cp ./dist/blaxel_darwin_arm64_v8.0/blaxel ~/.local/bin/blaxel; \
		fi; \
		cp ~/.local/bin/blaxel ~/.local/bin/bl; \
	fi

# Development build without goreleaser
build-dev:
	@echo "🔨 Development build with commit: $(GIT_COMMIT_SHORT)"
	go build -ldflags "-X main.version=dev -X main.commit=$(GIT_COMMIT) -X main.date=$(shell date -u +%Y-%m-%dT%H:%M:%SZ) -X main.sentryDSN=$(SENTRY_DSN)" -o ./bin/blaxel ./
	cp ./bin/blaxel ~/.local/bin/blaxel;
	cp ~/.local/bin/blaxel ~/.local/bin/bl;
	rm -r ./bin;
	@echo "✅ Binary built: ./bin/blaxel"

doc:
	rm -rf docs
	go run main.go docs --format=markdown --output=docs

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
	brew install goreleaser

tag:
	git tag -a v$(ARGS) -m "Release v$(ARGS)"
	git push origin v$(ARGS)

clean:
	rm -rf ./dist
	rm -rf ./bin

%:
	@:

.PHONY: test test-integration