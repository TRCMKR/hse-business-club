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

# Test infrastructure: back/docker-compose.test.yaml (Postgres on :5433, MinIO on
# :9000, data in tmpfs). I0 replaces it with the root compose file.
COMPOSE_TEST      := docker compose -f docker-compose.test.yaml
TEST_POSTGRES_DSN ?= postgres://hbc:hbc@localhost:5433/hbc?sslmode=disable
TEST_S3_ENDPOINT  ?= localhost:9000
# TEST_ENV=external: use already running infrastructure (TEST_POSTGRES_DSN,
# TEST_S3_ENDPOINT) instead of starting and stopping docker-compose.test.yaml.
TEST_ENV          ?= compose

TEST_INTEGRATION_CMD = TEST_POSTGRES_DSN="$(TEST_POSTGRES_DSN)" TEST_S3_ENDPOINT="$(TEST_S3_ENDPOINT)" \
	go test -race -count=1 -tags=integration ./...
TEST_E2E_CMD = TEST_POSTGRES_DSN="$(TEST_POSTGRES_DSN)" go test -race -count=1 -tags=e2e ./tests/e2e/...

# with_test_env runs $(2) with the compose services $(1) up (waiting for their
# healthchecks: pg_isready for Postgres) and always stops them afterwards,
# keeping the exit code of $(2).
ifeq ($(TEST_ENV),external)
with_test_env = $(2)
else
with_test_env = $(COMPOSE_TEST) up -d --wait $(1) && \
	{ status=0; ( $(2) ) || status=$$?; $(COMPOSE_TEST) down -v; exit $$status; }
endif

.PHONY: test
## runs unit tests (no infrastructure)
test:
	go test -race -count=1 ./...

.PHONY: test-integration
## runs integration tests (repositories, blob/s3, db) on Postgres + MinIO from docker-compose.test.yaml
test-integration:
	@$(call with_test_env,postgres minio,$(TEST_INTEGRATION_CMD))

.PHONY: test-e2e
## runs HTTP end-to-end tests (tests/e2e, app in-process) on Postgres from docker-compose.test.yaml
test-e2e:
	@$(call with_test_env,postgres,$(TEST_E2E_CMD))

.PHONY: test-all
## runs unit, integration and e2e tests (one Postgres + MinIO for both)
test-all: test
	@$(call with_test_env,postgres minio,$(TEST_INTEGRATION_CMD) && $(TEST_E2E_CMD))

.PHONY: test-env-up
## starts postgres and minio for tests and `make run`
test-env-up:
	$(COMPOSE_TEST) up -d --wait

.PHONY: test-env-down
## stops postgres and minio
test-env-down:
	$(COMPOSE_TEST) down -v

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
