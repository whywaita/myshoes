.PHONY: help
.DEFAULT_GOAL := help

CURRENT_REVISION = $(shell git rev-parse --short HEAD)
BUILD_LDFLAGS = "-X main.revision=$(CURRENT_REVISION)"

help:
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build All
	go generate ./...
	make build-proto
	go build -o myshoes -ldflags $(BUILD_LDFLAGS) cmd/server/cmd.go

build-linux: ## Build for Linux
	go generate ./...
	make build-proto
	GOOS=linux GOARCH=amd64 go build -o myshoes-linux-amd64 -ldflags $(BUILD_LDFLAGS) cmd/server/cmd.go

build-proto: ## Build proto file
	protoc -I ./api/proto/ --go_out=plugins=grpc:./api/proto/ ./api/proto/*.proto

test: ## Exec test
	go test -v ./...