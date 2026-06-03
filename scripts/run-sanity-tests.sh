#!/bin/bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

NAMESPACE=""
API_PORT=9000
DB_PORT=15432
BUILD_IMAGE=false
DEPLOY=false

# QUAY_REPO env var: e.g. quay.io/sgunta/kessel (no tag)
QUAY_REPO="${QUAY_REPO:-}"

usage() {
    cat <<EOF
Usage: $0 [options]

Modes:
  (no flags)       Run tests against an already-deployed environment
  -b               Build + push image, deploy, then test
  -d               Deploy whatever is in bonfire config, then test

Options:
  -n <namespace>   OpenShift namespace (default: auto-detect via 'oc project -q')
  -p <port>        Local port for API port-forward (default: 9000)
  -P <port>        Local port for DB port-forward (default: 15432)
  -h               Show this help

Environment variables:
  QUAY_REPO        Container image repo, e.g. quay.io/sgunta/kessel (required with -b)

Examples:
  # Tests only (environment already deployed)
  ./scripts/run-sanity-tests.sh

  # Build + push + deploy + test
  QUAY_REPO=quay.io/sgunta/kessel ./scripts/run-sanity-tests.sh -b

  # Deploy existing bonfire config + test
  ./scripts/run-sanity-tests.sh -d
EOF
    exit 1
}

while getopts "n:p:P:bdh" opt; do
    case $opt in
        n) NAMESPACE="$OPTARG" ;;
        p) API_PORT="$OPTARG" ;;
        P) DB_PORT="$OPTARG" ;;
        b) BUILD_IMAGE=true ;;
        d) DEPLOY=true ;;
        h) usage ;;
        *) usage ;;
    esac
done

# -b implies deploy since the new image needs to be rolled out
if [ "$BUILD_IMAGE" = true ]; then
    DEPLOY=true
fi

IMAGE_TAG=""

# --- Port management helpers ---

kill_port() {
    local port=$1
    local pids
    pids=$(lsof -t -i:"$port" 2>/dev/null || true)
    if [ -n "$pids" ]; then
        log_warn "Killing processes on port $port (pids: $pids)..."
        echo "$pids" | xargs kill 2>/dev/null || true
        sleep 1
        # Force-kill anything still hanging
        pids=$(lsof -t -i:"$port" 2>/dev/null || true)
        if [ -n "$pids" ]; then
            echo "$pids" | xargs kill -9 2>/dev/null || true
            sleep 1
        fi
    fi
}

wait_for_port() {
    local port=$1
    local timeout=${2:-15}
    for i in $(seq 1 "$timeout"); do
        if lsof -Pi :"$port" -sTCP:LISTEN -t >/dev/null 2>&1; then
            return 0
        fi
        sleep 1
    done
    return 1
}

# --- Cleanup (runs on EXIT, INT, TERM) ---

PIDS=()
cleanup() {
    log_info "Cleaning up..."
    for pid in "${PIDS[@]+"${PIDS[@]}"}"; do
        kill "$pid" 2>/dev/null || true
    done
    sleep 1
    # Force-kill any survivors
    for pid in "${PIDS[@]+"${PIDS[@]}"}"; do
        kill -9 "$pid" 2>/dev/null || true
    done
    # Ensure our ports are free
    kill_port "$API_PORT"
    kill_port "$DB_PORT"
    log_info "Cleanup complete"
}
trap cleanup EXIT INT TERM

# --- Phase 1: Build + Push (if -b) ---

if [ "$BUILD_IMAGE" = true ]; then
    if [ -z "$QUAY_REPO" ]; then
        log_error "QUAY_REPO env var is required when using -b (e.g. QUAY_REPO=quay.io/sgunta/kessel)"
        exit 1
    fi

    IMAGE_TAG="$(git rev-parse --short HEAD)-sanity"
    FULL_IMAGE="${QUAY_REPO}:${IMAGE_TAG}"

    log_info "Building image: $FULL_IMAGE (platform: linux/amd64)..."
    podman build --platform linux/amd64 \
        -f Dockerfile \
        -t "$FULL_IMAGE" \
        --build-arg GIT_COMMIT="$(git rev-parse HEAD)" \
        .

    log_info "Pushing image: $FULL_IMAGE..."
    podman push "$FULL_IMAGE"
    log_info "Image pushed successfully"

    # Update bonfire config with new image tag
    BONFIRE_CONFIG="$HOME/.config/bonfire/config.yaml"
    if [ -f "$BONFIRE_CONFIG" ]; then
        log_info "Updating bonfire config with IMAGE_TAG=$IMAGE_TAG, INVENTORY_IMAGE=$QUAY_REPO"
        python3 -c "
import yaml, sys
with open('$BONFIRE_CONFIG', 'r') as f:
    config = yaml.safe_load(f)
for app in config.get('apps', []):
    if app.get('name') == 'kessel':
        for comp in app.get('components', []):
            if comp.get('name') == 'kessel-inventory':
                comp.setdefault('parameters', {})
                comp['parameters']['INVENTORY_IMAGE'] = '$QUAY_REPO'
                comp['parameters']['IMAGE_TAG'] = '$IMAGE_TAG'
with open('$BONFIRE_CONFIG', 'w') as f:
    yaml.dump(config, f, default_flow_style=False, sort_keys=False)
" 2>/dev/null || {
            log_warn "python3+pyyaml not available, using sed for bonfire config update"
            sed -i.bak "s|IMAGE_TAG:.*|IMAGE_TAG: ${IMAGE_TAG}|" "$BONFIRE_CONFIG"
            sed -i.bak "s|INVENTORY_IMAGE:.*|INVENTORY_IMAGE: ${QUAY_REPO}|" "$BONFIRE_CONFIG"
            rm -f "${BONFIRE_CONFIG}.bak"
        }
        log_info "Bonfire config updated"
    else
        log_warn "Bonfire config not found at $BONFIRE_CONFIG, skipping update"
    fi
fi

# --- Phase 2: Deploy (if -b or -d) ---

if [ "$DEPLOY" = true ]; then
    log_info "Deploying kessel to ephemeral via bonfire..."

    BONFIRE_DEPLOY_ARGS="kessel"
    if [ -n "$NAMESPACE" ]; then
        BONFIRE_DEPLOY_ARGS="$BONFIRE_DEPLOY_ARGS -n $NAMESPACE"
    else
        CURRENT_USER="${USER:-$(whoami)}"
        RESERVED=$(bonfire namespace list 2>/dev/null | grep -E "^ephemeral-.*true.*${CURRENT_USER}" | awk '{print $1}' || true)
        if [ -n "$RESERVED" ]; then
            log_warn "Releasing existing reserved namespaces..."
            echo "$RESERVED" | while read -r ns; do
                bonfire namespace release "$ns" 2>&1 || true
            done
            sleep 3
        fi
    fi

    if [ -t 1 ]; then
        BONFIRE_OUTPUT=$(bonfire deploy $BONFIRE_DEPLOY_ARGS 2>&1 | tee /dev/tty)
    else
        BONFIRE_OUTPUT=$(bonfire deploy $BONFIRE_DEPLOY_ARGS 2>&1)
        echo "$BONFIRE_OUTPUT"
    fi
    DEPLOY_EXIT=$?
    if [ $DEPLOY_EXIT -ne 0 ]; then
        log_error "Bonfire deploy failed (exit code: $DEPLOY_EXIT)"
        exit 1
    fi

    NAMESPACE=$(echo "$BONFIRE_OUTPUT" | grep -oE "successfully deployed to namespace [a-z0-9-]+" | awk '{print $NF}' || true)
    if [ -z "$NAMESPACE" ]; then
        NAMESPACE=$(echo "$BONFIRE_OUTPUT" | tail -1 | tr -d '[:space:]')
    fi

    if [ -z "$NAMESPACE" ]; then
        log_error "Could not determine namespace from bonfire output"
        exit 1
    fi
    log_info "Deployed to namespace: $NAMESPACE"
fi

# --- Phase 3: Detect namespace ---

if [ -z "$NAMESPACE" ]; then
    NAMESPACE=$(oc project -q 2>/dev/null || true)
    if [ -z "$NAMESPACE" ]; then
        log_error "Could not detect namespace. Use -n <namespace>, -d to deploy, or set your oc project."
        exit 1
    fi
fi

log_info "Namespace: $NAMESPACE"

# --- Phase 4: Discover DB credentials ---

log_info "Discovering DB credentials..."
POSTGRES_USER=$(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.user}' | base64 -d)
POSTGRES_PASSWORD=$(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.password}' | base64 -d)
POSTGRES_DB=$(oc get secret kessel-inventory-db -n "$NAMESPACE" -o jsonpath='{.data.db\.name}' | base64 -d)

if [ -z "$POSTGRES_USER" ] || [ -z "$POSTGRES_PASSWORD" ] || [ -z "$POSTGRES_DB" ]; then
    log_error "Failed to read DB credentials from secret kessel-inventory-db"
    exit 1
fi
log_info "DB credentials discovered (user=$POSTGRES_USER, db=$POSTGRES_DB)"

# --- Phase 5: Port-forward + Test ---

# Free ports from any previous stale runs
kill_port "$API_PORT"
kill_port "$DB_PORT"

log_info "Starting API port-forward (localhost:$API_PORT -> svc/kessel-inventory-api:9000)..."
oc port-forward svc/kessel-inventory-api "$API_PORT":9000 -n "$NAMESPACE" &
PIDS+=($!)

log_info "Starting DB port-forward (localhost:$DB_PORT -> svc/kessel-inventory-db:5432)..."
oc port-forward svc/kessel-inventory-db "$DB_PORT":5432 -n "$NAMESPACE" &
PIDS+=($!)

log_info "Waiting for port-forwards to be ready..."
if ! wait_for_port "$API_PORT" 15; then
    log_error "API port $API_PORT not ready after 15s"
    exit 1
fi
if ! wait_for_port "$DB_PORT" 15; then
    log_error "DB port $DB_PORT not ready after 15s"
    exit 1
fi
log_info "Port-forwards ready"

log_info "Running sanity tests..."
echo "=============================================="

TEST_EXIT=0
INV_GRPC_URL="localhost:$API_PORT" \
POSTGRES_HOST=localhost \
POSTGRES_PORT="$DB_PORT" \
POSTGRES_USER="$POSTGRES_USER" \
POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
POSTGRES_DB="$POSTGRES_DB" \
go test -v -count=1 -tags=sanity ./test/e2e/sanity/ -timeout 10m || TEST_EXIT=$?

echo "=============================================="

if [ $TEST_EXIT -eq 0 ]; then
    log_info "All sanity tests PASSED"
else
    log_error "Sanity tests FAILED (exit code: $TEST_EXIT)"
fi

exit $TEST_EXIT
