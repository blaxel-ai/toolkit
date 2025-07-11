ARGS:= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

sdk:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	oapi-codegen -package=sdk \
		-generate=types,client,spec \
		-o=sdk/blaxel.go \
		-templates=./templates/go \
		definition.yml

build:
	goreleaser release --snapshot --clean --skip homebrew
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		if [ -d "./dist/blaxel_darwin_arm64" ]; then \
			cp ./dist/blaxel_darwin_arm64/blaxel ~/.local/bin/blaxel; \
		else \
			cp ./dist/blaxel_darwin_arm64_v8.0/blaxel ~/.local/bin/blaxel; \
		fi; \
		cp ~/.local/bin/blaxel ~/.local/bin/bl; \
	fi

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