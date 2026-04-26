.PHONY: build clean deps fmt help proto proto-check proto-clean test test-cover test-coverage test-coverage-check test-coverage-whole-repo test-coverage-whole-repo-check test-go test-race vet
	scripts.list script.run \
	buf.lint \
	proto.s3minio.update proto.s3minio.gen proto.s3minio.lint proto.s3minio.breaking proto.s3minio.check proto.service.scaffold \
	ci.check.direct-http ci.check.stub-drift \
	test-go test-coverage test-coverage-check coverage-inventory test-coverage-integration \
	test-coverage-handwritten test-coverage-check-handwritten coverage-inventory-handwritten

BUF ?= buf

# Merged coverage uses -coverpkg. Default excludes legacy phaudittrailv0 (dial-heavy,
# no deterministic -short coverage); override with COVERAGE_PKGS=./... to include it.
# Project goal is 90% merged statements; raise COVERAGE_MIN as coverage improves.
COVERAGE_GOAL ?= 90
COVERAGE_MIN ?= 65
COVERAGE_PKGS ?= $(shell go list ./... | grep -Fv '/phaudittrailv0' | grep -Fv '/sdk/shared/' | tr '\n' ',' | sed 's/,$$//')

# Handwritten-only merged coverage: excludes generated protobuf/wirepb packages from -coverpkg.
# Intended for confidence-driven targets like "85% handwritten coverage".
# Note: this excludes the *package* `sdk/services/s3minio/pb/wirepb` entirely.
COVERAGE_PKGS_HANDWRITTEN ?= $(shell go list ./... | \
	grep -Fv '/sdk/shared/' | \
	grep -Fv '/sdk/services/s3minio/pb/wirepb' | \
	tr '\n' ',' | sed 's/,$$//')

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

coverage-inventory: ## Merged coverage + grouped inventory (writes coverage.out, coverage-func.txt)
	@COVERAGE_PKGS=$(COVERAGE_PKGS) ./scripts/coverage-inventory.sh

test-coverage-handwritten: ## Print merged coverage (COVERAGE_PKGS_HANDWRITTEN); summary tail
	go test -count=1 -short -coverprofile=coverage.out -covermode=atomic \
		-coverpkg=$(COVERAGE_PKGS_HANDWRITTEN) \
		./...
	go tool cover -func=coverage.out | tail -25

test-coverage-check-handwritten: ## Fail if merged handwritten coverage < COVERAGE_MIN (goal COVERAGE_GOAL)
	@COVERAGE_MIN=$(COVERAGE_MIN) COVERAGE_GOAL=$(COVERAGE_GOAL) COVERAGE_PKGS=$(COVERAGE_PKGS_HANDWRITTEN) ./scripts/coverage-check.sh

coverage-inventory-handwritten: ## Merged handwritten coverage + grouped inventory (writes coverage.out, coverage-func.txt)
	@COVERAGE_PKGS=$(COVERAGE_PKGS_HANDWRITTEN) ./scripts/coverage-inventory.sh

test-coverage-integration: ## Same as test-coverage but without -short (optional CI / nightly)
	go test -count=1 -coverprofile=coverage-integration.out -covermode=atomic \
		-coverpkg=$(COVERAGE_PKGS) \
		./...
	@go tool cover -func=coverage-integration.out | tail -8

fmt: ## Run go fmt on all packages
	go fmt ./...

clean: ## Remove coverage output
	rm -f coverage.out coverage-func.txt coverage-integration.out

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

# --- Auto-generated required targets ---

proto: proto-clean ## Generate Go code from .proto files
	@echo "Proto generation..."
	@if [ -f ./protoc.sh ]; then chmod +x ./protoc.sh && ./protoc.sh; elif [ -f ./genproto.sh ]; then sh ./genproto.sh; else echo "No proto script found"; fi

proto-clean: ## Remove generated protobuf Go files before regeneration
	@echo "Proto clean..."
	@find . -name "*.pb.go" -type f -delete

proto-check: ## Fail if generated protobuf Go files are out of date
	@echo "Proto check..."
	@$(MAKE) proto
	@if ! git diff --quiet; then echo "Generated protobuf files are out of date. Commit them."; exit 1; fi

test-coverage-whole-repo: ## Print whole-repo merged coverage
	go test -count=1 -short -coverprofile=coverage.out -covermode=atomic -coverpkg=./... ./...
	go tool cover -func=coverage.out | tail -25

test-coverage-whole-repo-check: ## Fail if whole-repo merged coverage is below COVERAGE_MIN
	@echo "Checking whole-repo coverage against COVERAGE_MIN=$(COVERAGE_MIN)..."
	@go test -count=1 -short -coverprofile=coverage.out -covermode=atomic -coverpkg=./... ./... > /dev/null
	@total=$$(go tool cover -func=coverage.out | grep total | awk '{print substr($$3, 1, length($$3)-1)}') ; \
	awk -v total=$$total -v min=$(COVERAGE_MIN) 'BEGIN { if (total < min) { print "Whole-repo coverage " total "% < " min "%"; exit 1 } else { print "Whole-repo coverage " total "% >= " min "%"; exit 0 } }'

