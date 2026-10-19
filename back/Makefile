BINARY  := hbc
BIN_DIR := $(CURDIR)/bin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w \
	-X github.com/trcmkr/hse-business-club/back/internal/platform/telemetry.Version=$(VERSION) \
	-X github.com/trcmkr/hse-business-club/back/internal/platform/telemetry.Commit=$(COMMIT)

# Built with the local Go toolchain: a prebuilt binary older than the toolchain
# cannot type-check the standard library.
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2

ifneq (,$(wildcard .env))
	include .env
	export
endif

.PHONY: help
## prints help about all targets
help:
	@echo "Usage: make <target>"
	@echo ""
	@awk '/^## /{c=substr($$0,4);next} /^[a-zA-Z0-9_-]+:/{t=$$1;sub(":","",t);if(c!=""){printf "  %-20s %s\n",t,c;c=""}}' $(MAKEFILE_LIST)

.PHONY: tools
## installs protoc plugins from the go.mod tool section into ./bin
tools:
	GOBIN=$(BIN_DIR) go install tool

.PHONY: generate
## generates gRPC, gateway and swagger code from api/ into pkg/
generate: tools
	PATH="$(BIN_DIR):$$PATH" buf generate

.PHONY: lint
## runs buf lint and golangci-lint
lint:
	buf lint
	$(GOLANGCI_LINT) run ./...

.PHONY: test
## runs unit tests
test:
	go test -race -count=1 ./...

.PHONY: test-integration
## runs integration tests (needs docker-compose.test.yaml: make test-env-up)
test-integration:
	TEST_POSTGRES_DSN="$${TEST_POSTGRES_DSN:-postgres://hbc:hbc@localhost:5433/hbc?sslmode=disable}" \
	TEST_S3_ENDPOINT="$${TEST_S3_ENDPOINT:-localhost:9000}" \
	go test -race -count=1 -tags=integration ./...

.PHONY: test-e2e
## runs HTTP end-to-end tests (implemented in B5)
test-e2e:
	@if [ -d tests/e2e ]; then go test -count=1 -tags=e2e ./tests/e2e/...; \
	else echo "tests/e2e is not implemented yet (B5)"; fi

.PHONY: test-env-up
## starts postgres and minio for integration tests
test-env-up:
	docker compose -f ../docker-compose.yaml -f ../docker-compose.test.yaml up -d --wait db minio

.PHONY: test-env-down
## stops postgres and minio for integration tests
test-env-down:
	docker compose -f ../docker-compose.yaml -f ../docker-compose.test.yaml down -v

.PHONY: run
## runs the app locally (reads .env when APP_ENV=local)
run:
	go run -ldflags="$(LDFLAGS)" ./cmd/hbc

.PHONY: build
## builds the binary into ./build
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o ./build/$(BINARY) ./cmd/hbc

.PHONY: tidy
## runs go mod tidy
tidy:
	go mod tidy

.PHONY: migration-create
## creates a SQL migration: make migration-create ctx=users name=init
migration-create:
	@test -n "$(ctx)" || (echo "ctx is required: make migration-create ctx=users name=init" && exit 1)
	@test -n "$(name)" || (echo "name is required: make migration-create ctx=users name=init" && exit 1)
	go run github.com/pressly/goose/v3/cmd/goose@v3.27.0 -dir internal/$(ctx)/postgres/migrations -s create $(name) sql
