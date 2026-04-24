export PATH := $(PATH):$(shell go env GOPATH)/bin

BINARY   := codewalker
CMD      := ./cmd/codewalker
BIN_DIR  := bin
IMAGE    := codewalker:latest

.PHONY: proto build test lint clean docker/build docker/run ci

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
