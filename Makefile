.DEFAULT_GOAL := help

VERSION     ?= dev
VERSION_NUM  = $(patsubst v%,%,$(VERSION))
MODULE       = github.com/ppiankov/pgpulse

LDFLAGS = -s -w -X $(MODULE)/internal/cli.version=$(VERSION_NUM)

.PHONY: build test lint fmt docker verify all help

build: ## Build binary
	go build -ldflags="$(LDFLAGS)" -o bin/pgpulse ./cmd/pgpulse

test: ## Run tests with race detection
	go test -race -v ./internal/...

lint: ## Run golangci-lint
	golangci-lint run ./...

fmt: ## Format code
	gofmt -w cmd/ internal/

docker: ## Build Docker image
	docker build --build-arg VERSION=$(VERSION) -t pgpulse:$(VERSION_NUM) .

verify: build test lint ## Build, test, lint

all: fmt lint test build

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
