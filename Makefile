.PHONY: help deps build vet test test-race test-cover fmt clean

help: ## Display Makefile targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help

deps: ## Download and tidy module dependencies
	go mod download
	go mod tidy

build: ## Verify all packages compile
	go build ./...

vet: ## Run go vet on all packages
	go vet ./...

test: ## Run unit tests
	go test ./...

test-race: ## Run unit tests with race detector (recommended locally)
	go test -race ./...

test-cover: ## Run tests with coverage profile (coverage.out)
	go test ./... -coverprofile=coverage.out -covermode=atomic
	@go tool cover -func=coverage.out

fmt: ## Run go fmt on all packages
	go fmt ./...

clean: ## Remove coverage output
	rm -f coverage.out
