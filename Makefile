.PHONY: help deps build vet test test-race test-cover fmt clean \
	scripts.list script.run \
	buf.lint \
	proto.s3minio.update proto.s3minio.gen proto.s3minio.lint proto.s3minio.breaking proto.s3minio.check proto.service.scaffold \
	ci.check.direct-http ci.check.stub-drift \
	test-go test-coverage test-coverage-check

BUF ?= buf

# Merged coverage uses -coverpkg; default is the whole module. Project goal is
# 90% merged statements; current baseline is lower — raise COVERAGE_MIN over time.
COVERAGE_GOAL ?= 90
COVERAGE_MIN ?= 42
COVERAGE_PKGS ?= ./...

GOTOOLCHAIN_VAL := $(shell awk '/^toolchain /{print $$2; exit}' go.mod)
ifneq ($(GOTOOLCHAIN_VAL),)
  export GOTOOLCHAIN := $(GOTOOLCHAIN_VAL)
endif

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

test-go: ## Run Go unit tests (short mode; no race)
	go test -count=1 -short ./...

test-coverage: ## Print merged coverage (COVERAGE_PKGS); summary tail
	go test -count=1 -short -coverprofile=coverage.out -covermode=atomic \
		-coverpkg=$(COVERAGE_PKGS) \
		./...
	go tool cover -func=coverage.out | tail -25

test-coverage-check: ## Fail if merged coverage < COVERAGE_MIN (goal COVERAGE_GOAL)
	@COVERAGE_MIN=$(COVERAGE_MIN) COVERAGE_GOAL=$(COVERAGE_GOAL) COVERAGE_PKGS=$(COVERAGE_PKGS) ./scripts/coverage-check.sh

fmt: ## Run go fmt on all packages
	go fmt ./...

clean: ## Remove coverage output
	rm -f coverage.out

scripts.list: ## List executable scripts under scripts/
	@find scripts -type f -name '*.sh' -print | sort

script.run: ## Run any script: make script.run SCRIPT=scripts/proto/check-stub-drift.sh
	@test -n "$(SCRIPT)" || { echo "SCRIPT is required"; exit 1; }
	@test -f "$(SCRIPT)" || { echo "Script not found: $(SCRIPT)"; exit 1; }
	@bash "$(SCRIPT)"

proto.s3minio.update: ## Update S3MinIO proto snapshot into SDK paths
	@bash scripts/proto/update-s3minio-proto.sh

buf.lint: ## Run buf lint from repo root (same as Bitbucket Pipelines; requires buf in PATH)
	@command -v $(BUF) >/dev/null 2>&1 || { echo "buf is required in PATH (e.g. go install github.com/bufbuild/buf/cmd/buf@v1.50.0)"; exit 1; }
	@$(BUF) lint && printf '%s\n' 'buf.lint: OK (no violations)'

proto.s3minio.gen: ## Validate S3MinIO SDK compiles against proto snapshot (hand-maintained pb/)
	@bash scripts/proto/gen-s3minio-client.sh

proto.s3minio.lint: ## Lint S3MinIO proto with buf (requires buf in PATH)
	@command -v $(BUF) >/dev/null 2>&1 || { echo "buf is required in PATH"; exit 1; }
	@$(BUF) lint sdk/services/s3minio/proto && printf '%s\n' 'proto.s3minio.lint: OK (no violations)'

proto.s3minio.breaking: ## Run buf breaking check for the S3MinIO proto module
	@command -v $(BUF) >/dev/null 2>&1 || { echo "buf is required in PATH"; exit 1; }
	@$(BUF) breaking sdk/services/s3minio/proto --against .git#branch=HEAD

proto.s3minio.check: ## Run lint plus stub/proto drift checks for S3MinIO foundation
	@$(MAKE) proto.s3minio.lint
	@bash scripts/proto/check-stub-drift.sh

proto.service.scaffold: ## Scaffold sdk/services/<service> structure: make proto.service.scaffold SERVICE=clientpg
	@test -n "$(SERVICE)" || { echo "SERVICE is required"; exit 1; }
	@bash scripts/proto/new-service-scaffold.sh "$(SERVICE)"

ci.check.direct-http: ## Ensure no direct internal s3minio HTTP usage outside approved adapters
	@bash scripts/check-no-direct-s3minio-http.sh

ci.check.stub-drift: ## Alias for stub drift check used by CI
	@bash scripts/proto/check-stub-drift.sh
