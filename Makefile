.DEFAULT_GOAL := help

GO ?= go
COMPOSE ?= docker compose
BIN_DIR ?= bin
GO_FILES := $(shell find . -type f -name '*.go' -not -path './vendor/*')

INTEGRATION_PROJECT ?= ginblade-integration
INTEGRATION_POSTGRES_PORT ?= 55432
INTEGRATION_REDIS_PORT ?= 56379
TEST_POSTGRES_DSN ?= postgres://app:app@127.0.0.1:$(INTEGRATION_POSTGRES_PORT)/app?sslmode=disable&connect_timeout=5
TEST_REDIS_ADDR ?= 127.0.0.1:$(INTEGRATION_REDIS_PORT)
TEST_REDIS_PASSWORD ?=
TEST_REDIS_CACHE_DB ?= 14
TEST_REDIS_QUEUE_DB ?= 15
export TEST_POSTGRES_DSN TEST_REDIS_ADDR TEST_REDIS_PASSWORD TEST_REDIS_CACHE_DB TEST_REDIS_QUEUE_DB

.PHONY: help fmt fmt-check tidy-check verify vet lint test test-race test-integration \
	integration-up integration-down build run worker migrate \
	docker-build compose-up compose-down compose-logs ci clean

help:
	@echo "Available targets:"
	@echo "  fmt                  Format Go source"
	@echo "  fmt-check            Fail when Go source is not formatted"
	@echo "  lint                 Run golangci-lint"
	@echo "  test                 Run unit tests"
	@echo "  test-race            Run unit tests with the race detector"
	@echo "  integration-up       Start isolated Postgres and Redis test services"
	@echo "  test-integration     Run tests against external Postgres and Redis"
	@echo "  integration-down     Remove only the isolated integration-test project"
	@echo "  build                Build API, worker, and migration binaries"
	@echo "  compose-up           Build and start the full local stack"
	@echo "  compose-down         Stop the full local stack"
	@echo "  ci                   Run the local CI quality checks"

fmt:
	gofmt -w $(GO_FILES)

fmt-check:
	@files="$$(gofmt -l $(GO_FILES))"; if [ -n "$$files" ]; then echo "Unformatted Go files:"; echo "$$files"; exit 1; fi

tidy-check:
	$(GO) mod tidy -diff

verify:
	$(GO) mod verify

vet:
	$(GO) vet ./...

lint:
	golangci-lint run --build-tags=integration

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

test-integration:
	$(GO) test -race -tags=integration -count=1 -timeout=2m ./tests/integration

integration-up:
	COMPOSE_PROJECT_NAME='$(INTEGRATION_PROJECT)' \
	POSTGRES_PORT='$(INTEGRATION_POSTGRES_PORT)' \
	REDIS_PORT='$(INTEGRATION_REDIS_PORT)' \
	$(COMPOSE) up -d --wait postgres redis

integration-down:
	COMPOSE_PROJECT_NAME='$(INTEGRATION_PROJECT)' \
	POSTGRES_PORT='$(INTEGRATION_POSTGRES_PORT)' \
	REDIS_PORT='$(INTEGRATION_REDIS_PORT)' \
	$(COMPOSE) down --volumes

build:
	mkdir -p $(BIN_DIR)
	$(GO) build -trimpath -o $(BIN_DIR)/api ./cmd/api
	$(GO) build -trimpath -o $(BIN_DIR)/worker ./cmd/worker
	$(GO) build -trimpath -o $(BIN_DIR)/migrate ./cmd/migrate

run:
	$(GO) run ./cmd/api

worker:
	$(GO) run ./cmd/worker

migrate:
	$(GO) run ./cmd/migrate

docker-build:
	docker build -t ginblade:local .

compose-up:
	$(COMPOSE) up --build -d --wait

compose-down:
	$(COMPOSE) down

compose-logs:
	$(COMPOSE) logs -f

ci: fmt-check tidy-check verify vet test-race build

clean:
	rm -rf ./bin
	rm -f ./coverage.out
