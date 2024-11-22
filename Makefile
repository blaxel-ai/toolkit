sdk:
	cp ../control-plane/api/api/definitions/controlplane.yml ./definition.yml
	oapi-codegen -package=sdk \
		-generate=types,client,spec \
		-o=sdk/beamlit.go \
		-templates=./templates \
		definition.yml

build:
	goreleaser release --snapshot --clean
	cp ./dist/beamlit_darwin_arm64/beamlit ./beamlit

dev:
	alias bl="go run main.go"

doc:
	go run main.go docs --format=markdown --output=docs
	rm docs/beamlit_completion_zsh.md docs/beamlit_completion_bash.md
	
lint:
	golangci-lint run

.PHONY: sdk