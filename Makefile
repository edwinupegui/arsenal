.PHONY: help build run test test-cover lint fmt tidy migrate-up migrate-down migrate-new sqlc clean install-tools

BIN      := arsenal
PKG      := github.com/edwinupegui/arsenal
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT   ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE     ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS  := -s -w \
	-X $(PKG)/internal/cli.Version=$(VERSION) \
	-X $(PKG)/internal/cli.Commit=$(COMMIT) \
	-X $(PKG)/internal/cli.Date=$(DATE)

GOOSE_BIN     := $(shell go env GOPATH)/bin/goose
SQLC_BIN      := $(shell go env GOPATH)/bin/sqlc
GORELEASER    := $(shell go env GOPATH)/bin/goreleaser
GOLANGCI_LINT := $(shell go env GOPATH)/bin/golangci-lint

DB ?= ./arsenal.db

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "Targets:\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

build: ## Build local binary
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/arsenal

run: build ## Build and run TUI
	./$(BIN)

test: ## Run unit tests
	go test ./... -race -count=1

test-cover: ## Run tests with coverage
	go test ./... -race -count=1 -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

lint: ## Run golangci-lint
	$(GOLANGCI_LINT) run ./...

fmt: ## Format code
	go fmt ./...
	goimports -w .

tidy: ## Tidy go.mod
	go mod tidy

migrate-up: ## Apply pending migrations to $(DB)
	$(GOOSE_BIN) -dir internal/migrations sqlite3 $(DB) up

migrate-down: ## Rollback last migration on $(DB)
	$(GOOSE_BIN) -dir internal/migrations sqlite3 $(DB) down

migrate-new: ## Create a new migration. Usage: make migrate-new NAME=add_foo
	$(GOOSE_BIN) -dir internal/migrations create $(NAME) sql

sqlc: ## Regenerate sqlc code
	$(SQLC_BIN) generate

clean: ## Remove build artifacts
	rm -f $(BIN) coverage.out coverage.html
	rm -rf dist/

install-tools: ## Install dev tools to $GOPATH/bin
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/goreleaser/goreleaser@latest
	go install golang.org/x/tools/cmd/goimports@latest
