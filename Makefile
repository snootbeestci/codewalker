export PATH := $(PATH):$(shell go env GOPATH)/bin

BINARY   := codewalker
CMD      := ./cmd/codewalker
BIN_DIR  := bin
IMAGE    := codewalker:latest

.PHONY: proto build test lint clean docker/build docker/run ci release-dry-run

## Print the current version
version:
	@git describe --tags --always --dirty 2>/dev/null || echo "dev"

## Generate protobuf/gRPC code from proto/
proto:
	buf generate

## Compile the server binary
build:
	CGO_ENABLED=1 go build -o $(BIN_DIR)/$(BINARY) $(CMD)

## Run all tests
test:
	go test ./...

## Lint proto + Go source
lint:
	buf lint
	go vet ./...

## Remove generated artefacts
clean:
	rm -rf $(BIN_DIR) gen/

## Build Docker image
docker/build:
	docker build -t $(IMAGE) -f deploy/Dockerfile .

## Start the stack via docker-compose
docker/run:
	docker-compose -f deploy/docker-compose.yml up

## Run the same checks as CI (no API calls, no container required)
ci: lint
	CGO_ENABLED=1 go build ./...
	CGO_ENABLED=1 go test ./...

## Validate the Gradle publish setup locally without publishing anything.
## Requires: buf, JDK 17+, GITHUB_ACTOR and GITHUB_TOKEN env vars (can be dummy values for dry run).
release-dry-run:
	@command -v buf    >/dev/null 2>&1 || { echo "buf not found"; exit 1; }
	@command -v gradle >/dev/null 2>&1 || { echo "gradle not found"; exit 1; }
	@echo "--- generating Kotlin stubs..."
	buf generate --template buf.gen.kotlin.yaml
	@echo "--- validating Gradle publish configuration..."
	RELEASE_VERSION=dry-run \
	GITHUB_ACTOR=$${GITHUB_ACTOR:-dry-run} \
	GITHUB_TOKEN=$${GITHUB_TOKEN:-dry-run} \
	gradle -p gradle publishToMavenLocal
	@echo "--- dry run complete. Stubs published to local Maven cache."
	@echo "--- check ~/.m2/repository/com/github/snootbeestci/codewalker-proto/"
