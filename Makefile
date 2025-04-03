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
	uv run ruff check --fix

install:
	uv pip install openapi-python-client

tag:
	git tag -a v$(ARGS) -m "Release v$(ARGS)"
	git push origin v$(ARGS)

%:
	@:

.PHONY: sdk