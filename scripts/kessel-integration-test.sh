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
INVENTORY_GRPC="localhost:9081"
SPICEDB_GRPC="localhost:50051"
SPICEDB_TOKEN="${SPICEDB_GRPC_PRESHARED_KEY:-foobar}"
RBAC_HTTP="localhost:9080"
KAFKA_BOOTSTRAP="localhost:9092"
KAFKA_CONNECT="localhost:8083"

# Test identity
ORG_ID="test-org-kessel-e2e"
X_RH_IDENTITY=$(echo -n "{\"identity\":{\"account_number\":\"12345\",\"org_id\":\"${ORG_ID}\",\"type\":\"User\",\"user\":{\"user_id\":\"test-user-1\",\"email\":\"test@example.com\",\"username\":\"testuser\",\"is_org_admin\":true}}}" | base64 | tr -d '\n')

# Test host UUIDs — unique per run to avoid tombstone conflicts on re-run.
# DeleteResource soft-deletes (tombstone=true), and re-reporting a tombstoned
# resource is an update that doesn't re-create the SpiceDB relationship.
RUN_ID=$(uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-' | head -c 12)
HOST_ID="e2e10000-0000-0000-0000-${RUN_ID}"
INSIGHTS_ID="e2e20000-0000-0000-0000-${RUN_ID}"
SUB_MGR_ID="e2e30000-0000-0000-0000-${RUN_ID}"

# ─── Step 0: Prerequisites ───────────────────────────────────────────────────

log_step "Step 0: Prerequisites Check"

MISSING=""
for cmd in kcat grpcurl jq curl zed; do
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
  echo "    zed:     go install github.com/authzed/zed/cmd/zed@latest"
  exit 1
fi
pass "All prerequisites installed"

# ─── Step 1: Service Health Checks ───────────────────────────────────────────

log_step "Step 1: Service Health Checks"

wait_for_http_service() {
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

wait_for_grpc_service() {
  local name="$1" address="$2" service="$3" retries="${4:-30}" interval="${5:-5}"
  for ((i=1; i<=retries; i++)); do
    if grpcurl -plaintext "$address" "$service" &>/dev/null; then
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

wait_for_spicedb() {
  local retries="${1:-30}" interval="${2:-5}"
  for ((i=1; i<=retries; i++)); do
    if zed schema read --endpoint "${SPICEDB_GRPC}" --token "${SPICEDB_TOKEN}" --insecure --skip-version-check &>/dev/null; then
      pass "SpiceDB is ready"
      return 0
    fi
    if [[ $i -eq 1 ]]; then
      log_info "Waiting for SpiceDB..."
    fi
    sleep "$interval"
  done
  fail "SpiceDB did not become ready after $((retries * interval))s"
  return 1
}

read_spicedb_relationships() {
  zed relationship read "$@" \
    --json \
    --endpoint "${SPICEDB_GRPC}" \
    --token "${SPICEDB_TOKEN}" \
    --insecure \
    --skip-version-check 2>&1
}

HEALTH_OK=true
wait_for_grpc_service "Inventory API" "${INVENTORY_GRPC}" "kessel.inventory.v1.KesselInventoryHealthService/GetReadyz" || HEALTH_OK=false
wait_for_spicedb || HEALTH_OK=false
wait_for_http_service "RBAC"          "http://${RBAC_HTTP}/metrics" || HEALTH_OK=false
wait_for_http_service "Kafka Connect" "http://${KAFKA_CONNECT}/connectors" || HEALTH_OK=false

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

# ─── Step 3: Verify Workspace Hierarchy in SpiceDB ───────────────────────────

log_step "Step 3: Verify Workspace Hierarchy in SpiceDB"

log_info "Polling SpiceDB for workspace relationships (up to 60s)..."

WS_TUPLES_FOUND=false
for ((i=1; i<=20; i++)); do
  if TUPLES_RESPONSE=$(read_spicedb_relationships rbac/workspace); then
    TUPLE_COUNT=$(jq -s '[.[] | select(.relationship)] | length' <<< "$TUPLES_RESPONSE")
    if [[ "$TUPLE_COUNT" -gt 0 ]]; then
      WS_TUPLES_FOUND=true
      break
    fi
  fi
  sleep 3
done

if [[ "$WS_TUPLES_FOUND" == "true" ]]; then
  pass "Found ${TUPLE_COUNT} workspace relationship(s) in SpiceDB"

  # Verify the default workspace has a parent relationship (default → root)
  if [[ -n "$DEFAULT_WORKSPACE_ID" && -n "$ROOT_WORKSPACE_ID" ]]; then
    DEFAULT_WS_RELATIONSHIP_COUNT=$(jq -s \
      --arg workspace_id "$DEFAULT_WORKSPACE_ID" \
      --arg root_workspace_id "$ROOT_WORKSPACE_ID" \
      '[.[] | select(
        .relationship.resource.objectType == "rbac/workspace" and
        .relationship.resource.objectId == $workspace_id and
        .relationship.relation == "t_parent" and
        .relationship.subject.object.objectType == "rbac/workspace" and
        .relationship.subject.object.objectId == $root_workspace_id
      )] | length' <<< "$TUPLES_RESPONSE")
    if [[ "$DEFAULT_WS_RELATIONSHIP_COUNT" -gt 0 ]]; then
      pass "Default workspace has parent relationship to root workspace"
    else
      log_warn "Default workspace parent relationship not found (may take a moment to replicate)"
    fi
  fi
else
  fail "No workspace relationships found in SpiceDB after 60s"
  echo "  Check that RBAC CDC pipeline is running (rbac-kafka-consumer, kafka-connect connectors)"
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

# ─── Step 6: Verify Resource-to-Workspace Relationship in SpiceDB ────────────

log_step "Step 6: Verify Resource-to-Workspace Relationship in SpiceDB"

log_info "Polling SpiceDB for the host-to-workspace relationship (up to 60s)..."

HOST_TUPLE_FOUND=false
for ((i=1; i<=20; i++)); do
  if HOST_TUPLES_RESPONSE=$(read_spicedb_relationships "hbi/host:${HOST_ID}" t_workspace "rbac/workspace:${DEFAULT_WORKSPACE_ID}") &&
    HOST_RELATIONSHIP_COUNT=$(jq -s '[.[] | select(.relationship)] | length' <<< "$HOST_TUPLES_RESPONSE") &&
    [[ "$HOST_RELATIONSHIP_COUNT" -gt 0 ]]; then
    HOST_TUPLE_FOUND=true
    break
  fi
  sleep 3
done

if [[ "$HOST_TUPLE_FOUND" == "true" ]]; then
  pass "Host resource is linked to workspace ${DEFAULT_WORKSPACE_ID} in SpiceDB"
else
  fail "Host-to-workspace relationship not found in SpiceDB after 60s"
  echo "  The resource may have been stored without creating an authorization relationship."
  echo "  Check that authz.impl=spicedb in the inventory-api config."
fi

# ─── Step 7: RBAC Access Check ──────────────────────────────────────────────

log_step "Step 7: RBAC Access Check"

log_info "Polling RBAC V2 roles (up to 60s for CDC replication)..."

ROLES_FOUND=false
for ((i=1; i<=20; i++)); do
  ROLES_RESPONSE=$(curl -sf \
    -H "x-rh-identity: ${X_RH_IDENTITY}" \
    "http://${RBAC_HTTP}/api/rbac/v2/roles/" 2>&1) || true

  if [[ -n "${ROLES_RESPONSE:-}" ]]; then
    ROLE_COUNT=$(echo "$ROLES_RESPONSE" | jq '.data | length')
    if [[ "$ROLE_COUNT" -gt 0 ]]; then
      ROLES_FOUND=true
      break
    fi
  fi
  sleep 3
done

if [[ "$ROLES_FOUND" == "true" ]]; then
  pass "RBAC V2 has ${ROLE_COUNT} roles for tenant"
else
  fail "No roles found via RBAC V2 after 60s"
fi

# Verify workspaces list includes both root and default (V2)
WORKSPACES_RESPONSE=$(curl -sf \
  -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "http://${RBAC_HTTP}/api/rbac/v2/workspaces/" 2>&1) || {
  fail "RBAC V2 workspaces list request failed"
}

if [[ -n "${WORKSPACES_RESPONSE:-}" ]]; then
  WS_COUNT=$(echo "$WORKSPACES_RESPONSE" | jq '.meta.count // 0')
  if [[ "$WS_COUNT" -ge 2 ]]; then
    pass "RBAC V2 lists ${WS_COUNT} workspaces (root + default + any user-created)"
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

# Wait for tuple cleanup via CDC pipeline
log_info "Waiting for tuple cleanup (up to 30s)..."

TUPLE_CLEANED=false
for ((i=1; i<=10; i++)); do
  sleep 3
  if HOST_TUPLES_AFTER=$(read_spicedb_relationships "hbi/host:${HOST_ID}" t_workspace "rbac/workspace:${DEFAULT_WORKSPACE_ID}") &&
    REMAINING=$(jq -s '[.[] | select(.relationship)] | length' <<< "$HOST_TUPLES_AFTER") &&
    [[ "$REMAINING" -eq 0 ]]; then
    TUPLE_CLEANED=true
    break
  fi
done

if [[ "$TUPLE_CLEANED" == "true" ]]; then
  pass "Host-to-workspace relationship removed from SpiceDB after deletion"
else
  fail "Host-to-workspace relationship still present in SpiceDB after deletion"
fi

# ─── Step 9: Relationship Delta Endpoints ────────────────────────────────────

log_step "Step 9: Relationship Delta Endpoints (Add/RemoveResourceRelationships)"

DELTA_WORKSPACE_ID="delta-integration-$(uuidgen)"
DELTA_SERVICE_ID="11111111-1111-1111-1111-111111111111"

log_info "Auto-creating a workspace with a service preference via AddResourceRelationships"
ADD_DELTA_RESPONSE=$(grpcurl -plaintext -d @ "${INVENTORY_GRPC}" kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
  "type": "workspace",
  "reporterType": "features",
  "reporterInstanceId": "integration-test",
  "localResourceId": "${DELTA_WORKSPACE_ID}",
  "reporter": {
    "direct_service_preferences": ["${DELTA_SERVICE_ID}"]
  }
}
EOF
) || {
  fail "AddResourceRelationships request failed"
  echo "  Response: ${ADD_DELTA_RESPONSE}"
}

if [[ -n "${ADD_DELTA_RESPONSE}" ]]; then
  pass "Workspace auto-created with service preference via AddResourceRelationships"
fi

log_info "Polling SpiceDB for the preference relationship (up to 60s)..."
DELTA_TUPLE_FOUND=false
for ((i=1; i<=20; i++)); do
  if DELTA_TUPLES_RESPONSE=$(read_spicedb_relationships "rbac/workspace:${DELTA_WORKSPACE_ID}" t_features_workspace_direct_service_preferences "features/service:${DELTA_SERVICE_ID}") &&
    DELTA_RELATIONSHIP_COUNT=$(jq -s '[.[] | select(.relationship)] | length' <<< "$DELTA_TUPLES_RESPONSE") &&
    [[ "$DELTA_RELATIONSHIP_COUNT" -gt 0 ]]; then
    DELTA_TUPLE_FOUND=true
    break
  fi
  sleep 3
done

if [[ "$DELTA_TUPLE_FOUND" == "true" ]]; then
  pass "Service preference relationship created in SpiceDB by AddResourceRelationships"
else
  fail "Service preference relationship not found in SpiceDB after 60s"
fi

log_info "Removing the service preference via RemoveResourceRelationships"
REMOVE_DELTA_RESPONSE=$(grpcurl -plaintext -d @ "${INVENTORY_GRPC}" kessel.inventory.v1beta2.KesselInventoryService/RemoveResourceRelationships <<EOF
{
  "type": "workspace",
  "reporterType": "features",
  "reporterInstanceId": "integration-test",
  "localResourceId": "${DELTA_WORKSPACE_ID}",
  "reporter": {
    "direct_service_preferences": ["${DELTA_SERVICE_ID}"]
  }
}
EOF
) || {
  fail "RemoveResourceRelationships request failed"
  echo "  Response: ${REMOVE_DELTA_RESPONSE}"
}

if [[ -n "${REMOVE_DELTA_RESPONSE}" ]]; then
  pass "Service preference removed via RemoveResourceRelationships"
fi

log_info "Polling SpiceDB for the preference relationship removal (up to 60s)..."
DELTA_TUPLE_REMOVED=false
for ((i=1; i<=20; i++)); do
  if DELTA_TUPLES_RESPONSE=$(read_spicedb_relationships "rbac/workspace:${DELTA_WORKSPACE_ID}" t_features_workspace_direct_service_preferences "features/service:${DELTA_SERVICE_ID}") &&
    DELTA_REMAINING_COUNT=$(jq -s '[.[] | select(.relationship)] | length' <<< "$DELTA_TUPLES_RESPONSE") &&
    [[ "$DELTA_REMAINING_COUNT" -eq 0 ]]; then
    DELTA_TUPLE_REMOVED=true
    break
  fi
  sleep 3
done

if [[ "$DELTA_TUPLE_REMOVED" == "true" ]]; then
  pass "Service preference relationship removed from SpiceDB by RemoveResourceRelationships"
else
  fail "Service preference relationship still present in SpiceDB after 60s"
fi

log_info "Deleting the delta test workspace"
grpcurl -plaintext -d @ "${INVENTORY_GRPC}" kessel.inventory.v1beta2.KesselInventoryService/DeleteResource <<EOF
{
  "reference": {
    "resourceType": "workspace",
    "resourceId": "${DELTA_WORKSPACE_ID}",
    "reporter": {
      "type": "features",
      "instanceId": "integration-test"
    }
  }
}
EOF
pass "Delta test workspace deleted"

# ─── Step 10: Summary ─────────────────────────────────────────────────────────

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
