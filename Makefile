ARGS:= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

sdk:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	oapi-codegen -package=sdk \
		-generate=types,client,spec \
		-o=sdk/beamlit.go \
		-templates=./templates/go \
		definition.yml

sdk-python:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	rm -rf sdk-python/src/beamlit/api sdk-python/src/beamlit/models
	openapi-python-client generate \
		--path=definition.yml \
		--output-path=./tmp-sdk-python \
		--overwrite \
		--custom-template-path=./templates/python \
		--config=./config/openapi-python-client.yml
	cp -r ./tmp-sdk-python/beamlit/* ./sdk-python/src/beamlit/
	rm -rf ./tmp-sdk-python
	cd sdk-python && uv run ruff check --fix

sdk-ts:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	rm -rf sdk-ts/src/client/types.gen.ts sdk-ts/src/client/sdk.gen.ts
	npx @hey-api/openapi-ts@0.61.0 -i ./definition.yml -o ./tmp/sdk-ts -c @hey-api/client-fetch
	cp -r ./tmp/sdk-ts/* ./sdk-ts/src/client
	rm -rf ./tmp/sdk-ts

build:
	goreleaser release --snapshot --clean --skip homebrew
	@if [ "$(shell uname -s)" = "Darwin" ]; then \
		if [ -d "./dist/beamlit_darwin_arm64" ]; then \
			cp ./dist/beamlit_darwin_arm64/beamlit ~/.local/bin/beamlit; \
		else \
			cp ./dist/beamlit_darwin_arm64_v8.0/beamlit ~/.local/bin/beamlit; \
		fi; \
		cp ~/.local/bin/beamlit ~/.local/bin/bl; \
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
	git tag v$(ARGS)
	git push origin v$(ARGS)

%:
	@:

.PHONY: sdk sdk-python sdk-ts