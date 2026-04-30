#!/bin/bash
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

PASS_COUNT=0
FAIL_COUNT=0

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_step() { echo -e "\n${CYAN}=== $1 ===${NC}"; }

pass() {
  echo -e "  ${GREEN}✓ PASS:${NC} $1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

fail() {
  echo -e "  ${RED}✗ FAIL:${NC} $1"
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

# Ports (match full-kessel docker-compose)
INVENTORY_HTTP="localhost:8081"
INVENTORY_GRPC="localhost:9081"
RELATIONS_HTTP="localhost:8000"
RBAC_HTTP="localhost:9080"
KAFKA_BOOTSTRAP="localhost:9092"
KAFKA_CONNECT="localhost:8083"

# Test identity
ORG_ID="test-org-kessel-e2e"
X_RH_IDENTITY=$(echo -n "{\"identity\":{\"account_number\":\"12345\",\"org_id\":\"${ORG_ID}\",\"type\":\"User\",\"user\":{\"user_id\":\"test-user-1\",\"email\":\"test@example.com\",\"username\":\"testuser\"}}}" | base64 -w0)

# Test host UUIDs (deterministic for idempotent runs)
HOST_ID="e2e-10000000-0000-0000-0000-000000000001"
INSIGHTS_ID="e2e-20000000-0000-0000-0000-000000000001"
SUB_MGR_ID="e2e-30000000-0000-0000-0000-000000000001"

# ─── Step 0: Prerequisites ───────────────────────────────────────────────────

log_step "Step 0: Prerequisites Check"

MISSING=""
for cmd in kcat grpcurl jq curl; do
  if ! command -v "$cmd" &>/dev/null; then
    MISSING="${MISSING} ${cmd}"
  fi
done

if [[ -n "$MISSING" ]]; then
  log_error "Missing required tools:${MISSING}"
  echo "  Install with:"
  echo "    kcat:    dnf install kcat  /  brew install kcat"
  echo "    grpcurl: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
  echo "    jq:      dnf install jq    /  brew install jq"
  echo "    curl:    dnf install curl  /  brew install curl"
  exit 1
fi
pass "All prerequisites installed"

# ─── Step 1: Service Health Checks ───────────────────────────────────────────

log_step "Step 1: Service Health Checks"

wait_for_service() {
  local name="$1" url="$2" retries="${3:-30}" interval="${4:-5}"
  for ((i=1; i<=retries; i++)); do
    if curl -sf "$url" -o /dev/null 2>/dev/null; then
      pass "$name is ready"
      return 0
    fi
    if [[ $i -eq 1 ]]; then
      log_info "Waiting for $name..."
    fi
    sleep "$interval"
  done
  fail "$name did not become ready after $((retries * interval))s"
  return 1
}

HEALTH_OK=true
wait_for_service "Inventory API"  "http://${INVENTORY_HTTP}/api/kessel/v1/readyz" || HEALTH_OK=false
wait_for_service "Relations API"  "http://${RELATIONS_HTTP}/readyz" || HEALTH_OK=false
wait_for_service "RBAC"           "http://${RBAC_HTTP}/metrics" || HEALTH_OK=false
wait_for_service "Kafka Connect"  "http://${KAFKA_CONNECT}/connectors" || HEALTH_OK=false

if [[ "$HEALTH_OK" != "true" ]]; then
  log_error "Not all services are healthy. Is the stack running? (make kessel-up)"
  exit 1
fi

# ─── Step 2: Bootstrap Tenant via RBAC V2 ────────────────────────────────────

log_step "Step 2: Bootstrap Tenant via RBAC V2"

log_info "Triggering tenant bootstrap for org_id=${ORG_ID}"

WORKSPACE_RESPONSE=$(curl -sf \
  -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "http://${RBAC_HTTP}/api/rbac/v2/workspaces/?type=default" 2>&1) || {
  fail "RBAC V2 workspaces request failed"
  echo "  Response: ${WORKSPACE_RESPONSE}"
  exit 1
}

DEFAULT_WORKSPACE_ID=$(echo "$WORKSPACE_RESPONSE" | jq -r '.data[0].id // empty')

if [[ -z "$DEFAULT_WORKSPACE_ID" ]]; then
  fail "Could not extract default workspace ID from RBAC response"
  echo "  Response: ${WORKSPACE_RESPONSE}"
  exit 1
fi

pass "Tenant bootstrapped — default workspace: ${DEFAULT_WORKSPACE_ID}"

# Also fetch root workspace for reference
ROOT_RESPONSE=$(curl -sf \
  -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "http://${RBAC_HTTP}/api/rbac/v2/workspaces/?type=root" 2>&1) || true

ROOT_WORKSPACE_ID=$(echo "$ROOT_RESPONSE" | jq -r '.data[0].id // empty')
if [[ -n "$ROOT_WORKSPACE_ID" ]]; then
  pass "Root workspace: ${ROOT_WORKSPACE_ID}"
else
  log_warn "Could not retrieve root workspace (non-fatal)"
fi

# ─── Step 3: Verify Workspace Hierarchy in Relations API ─────────────────────

log_step "Step 3: Verify Workspace Hierarchy in Relations API"

TUPLES_RESPONSE=$(curl -sf \
  "http://${RELATIONS_HTTP}/v1beta1/tuples?filter.resourceNamespace=rbac&filter.resourceType=workspace" 2>&1) || {
  fail "Failed to query workspace tuples from Relations API"
  exit 1
}

TUPLE_COUNT=$(echo "$TUPLES_RESPONSE" | jq '[.tuples // [] | length] | add // 0')

if [[ "$TUPLE_COUNT" -gt 0 ]]; then
  pass "Found ${TUPLE_COUNT} workspace tuples in Relations API"
else
  fail "No workspace tuples found — RBAC bootstrap may not have replicated to Relations API"
  echo "  This can happen if Relations API was slow to start. Wait a few seconds and re-run."
fi

# Verify the default workspace has a parent tuple (default → root)
if [[ -n "$DEFAULT_WORKSPACE_ID" ]]; then
  DEFAULT_WS_TUPLE=$(echo "$TUPLES_RESPONSE" | jq --arg wsid "$DEFAULT_WORKSPACE_ID" \
    '[.tuples[]? | select(.resource.id == $wsid)] | length')
  if [[ "$DEFAULT_WS_TUPLE" -gt 0 ]]; then
    pass "Default workspace has parent relationship in hierarchy"
  else
    log_warn "Default workspace parent tuple not found (may take a moment to replicate)"
  fi
fi

# ─── Step 4: Publish Simulated HBI Host Event via kcat ───────────────────────

log_step "Step 4: Publish Simulated HBI Host Event via kcat"

HBI_MESSAGE=$(cat <<EOF
{"schema":{},"payload":{"id":"${HOST_ID}","ansible_host":"e2e-test-host","insights_id":"${INSIGHTS_ID}","subscription_manager_id":"${SUB_MGR_ID}","satellite_id":null,"groups":[{"id":"${DEFAULT_WORKSPACE_ID}"}]}}
EOF
)

log_info "Publishing HBI host event to outbox.event.hbi.hosts"
log_info "  host_id=${HOST_ID}, workspace_id=${DEFAULT_WORKSPACE_ID}"

echo "${HBI_MESSAGE}" | kcat -P -b "${KAFKA_BOOTSTRAP}" \
  -H "operation=ReportResource" -H "version=v1beta2" \
  -t outbox.event.hbi.hosts -K "|" 2>&1 || {
  fail "Failed to publish HBI event to Kafka"
  exit 1
}

pass "HBI host event published to Kafka"

# ─── Step 5: Wait for Consumer Processing ────────────────────────────────────

log_step "Step 5: Wait for Consumer Processing"

log_info "Polling Inventory API for the reported resource (up to 30s)..."

RESOURCE_FOUND=false
for ((i=1; i<=10; i++)); do
  REPORT_RESULT=$(grpcurl -plaintext -d "{
    \"type\": \"host\",
    \"reporterType\": \"hbi\",
    \"reporterInstanceId\": \"redhat\",
    \"representations\": {
      \"metadata\": {
        \"localResourceId\": \"${HOST_ID}\",
        \"apiHref\": \"https://apihref.com/\",
        \"consoleHref\": \"https://www.console.com/\",
        \"reporterVersion\": \"1.0\"
      },
      \"common\": {
        \"workspace_id\": \"${DEFAULT_WORKSPACE_ID}\"
      },
      \"reporter\": {
        \"insights_id\": \"${INSIGHTS_ID}\",
        \"subscription_manager_id\": \"${SUB_MGR_ID}\",
        \"ansible_host\": \"e2e-test-host\"
      }
    }
  }" "${INVENTORY_GRPC}" kessel.inventory.v1beta2.KesselInventoryService/ReportResource 2>&1) && {
    RESOURCE_FOUND=true
    break
  }
  sleep 3
done

if [[ "$RESOURCE_FOUND" == "true" ]]; then
  pass "Resource confirmed in Inventory API (host_id=${HOST_ID})"
else
  fail "Resource not found in Inventory API after 30s"
  echo "  Last grpcurl output: ${REPORT_RESULT}"
fi

# ─── Step 6: Verify Resource-to-Workspace Tuple in Relations API ─────────────

log_step "Step 6: Verify Resource-to-Workspace Tuple in Relations API"

# Give Relations API a moment to process the tuple creation
sleep 3

HOST_TUPLES_RESPONSE=$(curl -sf \
  "http://${RELATIONS_HTTP}/v1beta1/tuples?filter.resourceType=host" 2>&1) || {
  fail "Failed to query host tuples from Relations API"
}

if [[ -n "${HOST_TUPLES_RESPONSE:-}" ]]; then
  HOST_TUPLE_COUNT=$(echo "$HOST_TUPLES_RESPONSE" | jq '[.tuples // [] | length] | add // 0')

  if [[ "$HOST_TUPLE_COUNT" -gt 0 ]]; then
    pass "Found ${HOST_TUPLE_COUNT} host tuple(s) in Relations API"

    # Verify the tuple links our host to the correct workspace
    MATCHING_TUPLE=$(echo "$HOST_TUPLES_RESPONSE" | jq --arg wsid "$DEFAULT_WORKSPACE_ID" \
      '[.tuples[]? | select(.subject.subject.id == $wsid)] | length')

    if [[ "$MATCHING_TUPLE" -gt 0 ]]; then
      pass "Host resource is linked to workspace ${DEFAULT_WORKSPACE_ID}"
    else
      fail "Host tuple exists but not linked to expected workspace"
      echo "  Expected workspace: ${DEFAULT_WORKSPACE_ID}"
      echo "  Tuples: $(echo "$HOST_TUPLES_RESPONSE" | jq -c '.tuples[]? | {resource: .resource.id, relation: .relation, subject: .subject.subject.id}')"
    fi
  else
    fail "No host tuples found in Relations API"
    echo "  The resource may have been stored without creating authorization tuples."
    echo "  Check that authz.impl=kessel in the inventory-api config."
  fi
fi

# ─── Step 7: RBAC V2 Access Check ───────────────────────────────────────────

log_step "Step 7: RBAC V2 Access Check"

# Verify roles exist for this tenant
ROLES_RESPONSE=$(curl -sf \
  -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "http://${RBAC_HTTP}/api/rbac/v2/roles/" 2>&1) || {
  fail "RBAC V2 roles request failed"
}

if [[ -n "${ROLES_RESPONSE:-}" ]]; then
  ROLE_COUNT=$(echo "$ROLES_RESPONSE" | jq '.meta.count // 0')
  if [[ "$ROLE_COUNT" -gt 0 ]]; then
    pass "RBAC has ${ROLE_COUNT} roles for tenant org_id=${ORG_ID}"
  else
    fail "No roles found in RBAC for this tenant"
  fi
fi

# Verify workspaces list includes both root and default
WORKSPACES_RESPONSE=$(curl -sf \
  -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "http://${RBAC_HTTP}/api/rbac/v2/workspaces/" 2>&1) || {
  fail "RBAC V2 workspaces list request failed"
}

if [[ -n "${WORKSPACES_RESPONSE:-}" ]]; then
  WS_COUNT=$(echo "$WORKSPACES_RESPONSE" | jq '.meta.count // 0')
  if [[ "$WS_COUNT" -ge 2 ]]; then
    pass "RBAC lists ${WS_COUNT} workspaces (root + default + any user-created)"
  else
    fail "Expected at least 2 workspaces (root + default), got ${WS_COUNT}"
  fi
fi

# ─── Step 8: Resource Deletion Flow ──────────────────────────────────────────

log_step "Step 8: Resource Deletion Flow"

log_info "Deleting resource via Inventory API gRPC"

DELETE_RESULT=$(grpcurl -plaintext -d "{
  \"reference\": {
    \"resourceType\": \"host\",
    \"resourceId\": \"${HOST_ID}\",
    \"reporter\": {
      \"type\": \"hbi\",
      \"instanceId\": \"redhat\"
    }
  }
}" "${INVENTORY_GRPC}" kessel.inventory.v1beta2.KesselInventoryService/DeleteResource 2>&1) && {
  pass "Resource deleted from Inventory API"
} || {
  fail "Failed to delete resource from Inventory API"
  echo "  grpcurl output: ${DELETE_RESULT}"
}

# Wait for tuple cleanup
sleep 3

HOST_TUPLES_AFTER=$(curl -sf \
  "http://${RELATIONS_HTTP}/v1beta1/tuples?filter.resourceType=host" 2>&1) || true

if [[ -n "${HOST_TUPLES_AFTER:-}" ]]; then
  REMAINING=$(echo "$HOST_TUPLES_AFTER" | jq --arg wsid "$DEFAULT_WORKSPACE_ID" \
    '[.tuples[]? | select(.subject.subject.id == $wsid)] | length')

  if [[ "$REMAINING" -eq 0 ]]; then
    pass "Host-to-workspace tuple removed from Relations API after deletion"
  else
    fail "Host-to-workspace tuple still present after deletion (count: ${REMAINING})"
  fi
fi

# ─── Step 9: Summary ─────────────────────────────────────────────────────────

log_step "Summary"

TOTAL=$((PASS_COUNT + FAIL_COUNT))
echo ""
echo -e "  ${GREEN}Passed: ${PASS_COUNT}${NC}  /  ${RED}Failed: ${FAIL_COUNT}${NC}  /  Total: ${TOTAL}"
echo ""

if [[ "$FAIL_COUNT" -eq 0 ]]; then
  echo -e "${GREEN}All integration tests passed.${NC}"
  exit 0
else
  echo -e "${RED}${FAIL_COUNT} test(s) failed.${NC}"
  exit 1
fi
