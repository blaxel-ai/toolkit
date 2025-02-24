ARGS:= $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

sdk: sdk-go sdk-python sdk-ts

sdk-go:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	oapi-codegen -package=sdk \
		-generate=types,client,spec \
		-o=sdk/blaxel.go \
		-templates=./templates/go \
		definition.yml

sdk-python:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	rm -rf sdk-python/src/blaxel/api sdk-python/src/blaxel/models
	openapi-python-client generate \
		--path=definition.yml \
		--output-path=./tmp-sdk-python \
		--overwrite \
		--custom-template-path=./templates/python \
		--config=./config/openapi-python-client.yml
	cp -r ./tmp-sdk-python/blaxel/* ./sdk-python/src/blaxel/
	rm -rf ./tmp-sdk-python
	cd sdk-python && uv run ruff check --fix

sdk-ts:
	cp ../controlplane/api/api/definitions/controlplane.yml ./definition.yml
	rm -rf sdk-ts/src/client/types.gen.ts sdk-ts/src/client/sdk.gen.ts
	npx @hey-api/openapi-ts@0.61.0 -i ./definition.yml -o ./tmp/sdk-ts -c @hey-api/client-fetch
	cp -r ./tmp/sdk-ts/* ./sdk-ts/src/client

	sed -i.bak 's/from '\''\.\/sdk\.gen'\''/from '\''\.\/sdk\.gen\.js'\''/g' sdk-ts/src/client/index.ts
	sed -i.bak 's/from '\''\.\/types\.gen'\''/from '\''\.\/types\.gen\.js'\''/g' sdk-ts/src/client/index.ts
	sed -i.bak 's/from '\''\.\/types\.gen'\''/from '\''\.\/types\.gen\.js'\''/g' sdk-ts/src/client/sdk.gen.ts
	rm -f sdk-ts/src/client/index.ts.bak
	rm -f sdk-ts/src/client/sdk.gen.ts.bak
	rm -rf ./tmp/sdk-ts

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

doc: doc-go doc-python doc-ts

doc-go:
	rm -rf docs
	go run main.go docs --format=markdown --output=docs
	rm docs/bl_completion_zsh.md docs/bl_completion_bash.md

doc-python:
	rm -rf sdk-python/docs
	cd sdk-python && sh doc.sh

doc-ts:
	rm -rf sdk-ts/docs
	cd sdk-ts && pnpm run docs

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

.PHONY: sdk sdk-python sdk-ts