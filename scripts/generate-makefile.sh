#!/usr/bin/env bash
# generate-makefile.sh — Auto-detect Go service or library layout and emit Makefile + run.sh.
# Usage: ./scripts/generate-makefile.sh [--service-path DIR] [--dry-run]
# PayCloud pattern: help target, race-aware local dev, optional proto + cicd delegation.

set -euo pipefail

DRY_RUN=false
SERVICE_PATH=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --service-path)
      SERVICE_PATH="${2:?}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--service-path DIR] [--dry-run]"
      exit 0
      ;;
    *)
      echo "ERROR: unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$SERVICE_PATH" ]]; then
  SERVICE_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fi

cd "$SERVICE_PATH"

if [[ ! -f go.mod ]]; then
  echo "ERROR: go.mod not found in $SERVICE_PATH" >&2
  exit 1
fi

detect_entry_point() {
  local f
  for f in server.go main.go lets.go; do
    if [[ -f "$f" ]]; then
      echo "$f"
      return 0
    fi
  done
  if [[ -f cmd/main.go ]]; then
    echo "./cmd/main.go"
    return 0
  fi
  return 1
}

collect_proto_dirs() {
  local d found=""
  for d in grpc_server/models_grpc grpc_server/model_grpc proto proto/main services_grpc; do
    [[ -L "${d%/}" ]] && continue
    [[ -d "$d" ]] || continue
    if compgen -G "$d"/*.proto &>/dev/null || find "$d" -maxdepth 4 -name '*.proto' -print -quit | grep -q .; then
      found="$found $d"
    fi
  done
  echo "${found# }"
}

ENTRY_POINT=""
if detect_entry_point; then
  ENTRY_POINT="$(detect_entry_point)"
fi

PROTO_DIRS_RAW="$(collect_proto_dirs || true)"
read -r -a PROTO_DIRS <<< "${PROTO_DIRS_RAW:-}"

HAS_PROTOC_SH=false
[[ -f protoc.sh ]] && HAS_PROTOC_SH=true

HAS_CICD=false
[[ -x cicd/cicd ]] && HAS_CICD=true

HAS_DOCKERFILE=false
[[ -f Dockerfile ]] && HAS_DOCKERFILE=true

DUAL_ENGINE=false
if grep -qE 'github.com/jackc/pgx|github.com/lib/pq' go.mod 2>/dev/null && grep -qE 'github.com/go-sql-driver/mysql|gorm.io/driver/mysql' go.mod 2>/dev/null; then
  DUAL_ENGINE=true
fi

SERVICE_TYPE="library"
if [[ -n "$ENTRY_POINT" ]]; then
  SERVICE_TYPE="binary"
fi

MODULE_PATH=""
MODULE_PATH="$(head -1 go.mod | awk '{print $2}')"
SERVICE_NAME="$(basename "$SERVICE_PATH")"

echo "== generate-makefile: service_path=$SERVICE_PATH"
echo "   module=$MODULE_PATH type=$SERVICE_TYPE entry=${ENTRY_POINT:-<none>}"
echo "   proto_dirs=[${PROTO_DIRS_RAW:-}] cicd=$HAS_CICD docker=$HAS_DOCKERFILE dual_db=$DUAL_ENGINE"

write_makefile_library() {
  cat <<'MAKEFILE_EOF'
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
MAKEFILE_EOF
}

write_makefile_binary() {
  local entry="$1"
  local proto_block=""
  local cicd_block=""
  local db_block=""

  if [[ ${#PROTO_DIRS[@]} -gt 0 ]] && $HAS_PROTOC_SH; then
    proto_block=$(
      cat <<PROTO_PART
proto: ## Generate Go code from .proto (runs ./protoc.sh)
	@echo "Generating protobuf code..."
	@PATH="\$\$(go env GOPATH)/bin:\$\$PATH" ./protoc.sh
	@echo "Done."
PROTO_PART
    )
  elif [[ ${#PROTO_DIRS[@]} -gt 0 ]]; then
    proto_block=$(
      cat <<'PROTO_PART'
proto: ## Generate protobuf (add ./protoc.sh — proto files detected)
	@echo "ERROR: protoc.sh missing; add script or remove proto target."; exit 1
PROTO_PART
    )
  fi

  if $HAS_CICD; then
    cicd_block=$(
      cat <<'CICD_PART'

builder: ## Docker builder stage (cicd/cicd)
	./cicd/cicd builder

docker-build: ## Docker production image (cicd/cicd build)
	./cicd/cicd build

test-ci: ## Tests in CI/docker environment (cicd/cicd)
	./cicd/cicd t

release: ## Release image (cicd/cicd)
	./cicd/cicd release

deploy: ## Deploy (cicd/cicd)
	./cicd/cicd deploy
CICD_PART
    )
  fi

  if $DUAL_ENGINE; then
    db_block=$(
      cat <<'DB_PART'

db-validate: ## Document dual-engine DB support (MariaDB + PostgreSQL drivers in go.mod)
	@echo "Dual-engine drivers detected — validate migrations against both engines in CI."
DB_PART
    )
  fi

  local phony="help run run-debug deps build vet test test-race"
  [[ -n "$proto_block" ]] && phony="$phony proto"
  $HAS_CICD && phony="$phony builder docker-build test-ci release deploy"
  $DUAL_ENGINE && phony="$phony db-validate"

  cat <<MAKEFILE_HEAD
.PHONY: $phony

help: ## Display Makefile targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_.-]+:.*?## / {printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}' \$(MAKEFILE_LIST)

.DEFAULT_GOAL := help

run: ## Run service locally (race detector, readable paths)
	@echo "Running $SERVICE_NAME..."
	@go run -race -trimpath=false $entry

run-debug: ## Run with GODEBUG (troubleshooting)
	@echo "Running $SERVICE_NAME (GODEBUG)..."
	@GODEBUG=asyncpreemptoff=0 go run -race -trimpath=false $entry

deps: ## Download and tidy module dependencies
	go mod download
	go mod tidy

build: ## Verify all packages compile
	go build ./...

vet: ## Run go vet
	go vet ./...

test: ## Run unit tests
	go test ./...

test-race: ## Run unit tests with race detector
	go test -race ./...
MAKEFILE_HEAD

  if [[ -n "$proto_block" ]]; then
    echo ""
    echo "$proto_block"
  fi
  if [[ -n "$cicd_block" ]]; then
    echo "$cicd_block"
  fi
  if [[ -n "$db_block" ]]; then
    echo "$db_block"
  fi
}

write_run_sh_library() {
  cat <<'RUNSH_EOF'
#!/usr/bin/env bash
# run.sh — library module (no main): runs tests with race detector.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

DEBUG=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --debug) DEBUG=true; shift ;;
    -h|--help)
      echo "Usage: $0 [--debug] [--help]"
      echo "  Library repo: runs go test -race ./... (optionally GODEBUG with --debug)"
      exit 0
      ;;
    *) echo "ERROR: unknown option: $1" >&2; exit 1 ;;
  esac
done

echo "Running unit tests (library module, race detector)..."
if [[ "$DEBUG" == "true" ]]; then
  export GODEBUG=asyncpreemptoff=0
fi
exec go test -race ./...
RUNSH_EOF
}

write_run_sh_binary() {
  local entry="$1"
  cat <<RUNSH_EOF
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="\$(cd "\$(dirname "\${BASH_SOURCE[0]}")" && pwd)"
cd "\$SCRIPT_DIR"

DEBUG=false
while [[ \$# -gt 0 ]]; do
  case "\$1" in
    --debug) DEBUG=true; shift ;;
    -h|--help) echo "Usage: \$0 [--debug] [--help]"; exit 0 ;;
    *) echo "ERROR: unknown option: \$1" >&2; exit 1 ;;
  esac
done

MAIN_FILE="$entry"
if [[ ! -f "\$MAIN_FILE" ]]; then
  echo "ERROR: entry point not found: \$MAIN_FILE" >&2
  exit 1
fi

echo "Starting $SERVICE_NAME (race detector)..."
if [[ "\$DEBUG" == "true" ]]; then
  exec env GODEBUG=asyncpreemptoff=0 go run -race -trimpath=false "\$MAIN_FILE"
else
  exec go run -race -trimpath=false "\$MAIN_FILE"
fi
RUNSH_EOF
}

MAKEFILE_CONTENT=""
if [[ "$SERVICE_TYPE" == "library" ]]; then
  MAKEFILE_CONTENT="$(write_makefile_library)"
  RUNSH_CONTENT="$(write_run_sh_library)"
else
  MAKEFILE_CONTENT="$(write_makefile_binary)"
  RUNSH_CONTENT="$(write_run_sh_binary "$ENTRY_POINT")"
fi

if $DRY_RUN; then
  echo "---- Makefile (dry-run) ----"
  echo "$MAKEFILE_CONTENT"
  echo "---- run.sh (dry-run) ----"
  echo "$RUNSH_CONTENT"
  exit 0
fi

printf '%s\n' "$MAKEFILE_CONTENT" > Makefile
printf '%s\n' "$RUNSH_CONTENT" > run.sh
chmod +x run.sh

echo "Wrote $SERVICE_PATH/Makefile and $SERVICE_PATH/run.sh"

if command -v make >/dev/null 2>&1; then
  make -n help >/dev/null && echo "OK: make -n help"
fi
if command -v bash >/dev/null 2>&1; then
  bash -n run.sh && echo "OK: bash -n run.sh"
fi
