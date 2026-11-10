#!/bin/bash
#
# Sets up a small workspace hierarchy for local Features service testing
# against a running `make kessel-up` stack.
#
# The script talks only to the RBAC service: the first workspaces request
# bootstraps the tenant (creating the root and default workspaces), and each
# created workspace replicates into SpiceDB as a t_parent hierarchy link via
# CDC. The printed IDs are used as localResourceId values when reporting
# Features data through the Inventory API -- see
# docs/dev-guides/features-service-local-testing.md for the full walkthrough.
#
# Usage:
#   ./scripts/features-workspace-setup.sh [org_id]
#
# The workspace IDs are written to ./features-test-workspaces.env so the
# guide's examples can source them:
#   source ./features-test-workspaces.env
#
# The script is safe to re-run: existing workspaces are looked up by name
# instead of created again.

set -euo pipefail

RBAC_HTTP="${RBAC_HTTP:-localhost:9080}"
ORG_ID="${1:-features-local-test}"
OUT_FILE="${OUT_FILE:-./features-test-workspaces.env}"

# Restrict inputs to safe character sets: ORG_ID is embedded in the identity
# header and the generated env file, so anything outside [A-Za-z0-9_-] is
# rejected rather than escaped.
if [[ ! "$ORG_ID" =~ ^[A-Za-z0-9_-]+$ ]]; then
  echo "ERROR: org_id must contain only letters, digits, underscores and hyphens (got: ${ORG_ID})" >&2
  exit 1
fi

# The RBAC service identifies the caller (and the tenant) through the
# x-rh-identity header; the Inventory API gRPC calls in the guide need no
# authentication on this stack.
X_RH_IDENTITY=$(printf '{"identity":{"account_number":"12345","org_id":"%s","type":"User","user":{"user_id":"features-tester","email":"features@example.com","username":"features","is_org_admin":true}}}' "$ORG_ID" | base64 | tr -d '\n')

# find_workspace NAME prints the id of the first workspace with that name,
# or nothing when no workspace has it.
find_workspace() {
  curl -sf -H "x-rh-identity: ${X_RH_IDENTITY}" "${RBAC_HTTP}/api/rbac/v2/workspaces/" |
    jq -r --arg name "$1" '.data[] | select(.name == $name) | .id' | head -1
}

# create_workspace NAME PARENT_ID creates a workspace under the parent and
# prints the new id.
create_workspace() {
  curl -sf -X POST -H "x-rh-identity: ${X_RH_IDENTITY}" -H "Content-Type: application/json" \
    -d "$(jq -n --arg name "$1" --arg parent "$2" '{name: $name, parent_id: $parent}')" \
    "${RBAC_HTTP}/api/rbac/v2/workspaces/" | jq -r '.id'
}

# ensure_workspace NAME PARENT_ID prints the id of the named workspace,
# creating it under the parent when it does not exist yet.
ensure_workspace() {
  local name="$1" parent="$2" id
  id=$(find_workspace "$name") || true
  if [[ -n "${id:-}" ]]; then
    echo "  ${name}: already exists (${id})" >&2
  else
    id=$(create_workspace "$name" "$parent")
    echo "  ${name}: created (${id})" >&2
  fi
  echo "$id"
}

echo "Bootstrapping tenant for org_id=${ORG_ID} (this also creates the root and default workspaces)..."
DEFAULT_ID=$(curl -sf -H "x-rh-identity: ${X_RH_IDENTITY}" \
  "${RBAC_HTTP}/api/rbac/v2/workspaces/?type=default" | jq -r '.data[0].id')
if [[ -z "$DEFAULT_ID" ]]; then
  echo "ERROR: could not bootstrap the tenant -- is the stack running? (make kessel-up)" >&2
  exit 1
fi
echo "  default workspace: ${DEFAULT_ID}" >&2

echo "Ensuring the test hierarchy exists..."
DEPARTMENT_ID=$(ensure_workspace "Features Department" "$DEFAULT_ID")
TEAM_ID=$(ensure_workspace "Features Team" "$DEPARTMENT_ID")

# The workspace IDs are written into a sourceable env file, so they are
# validated as UUIDs before use: anything else could inject shell syntax
# when the file is sourced.
for id in "$DEFAULT_ID" "$DEPARTMENT_ID" "$TEAM_ID"; do
  if [[ ! "$id" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$ ]]; then
    echo "ERROR: refusing to write non-UUID workspace id to ${OUT_FILE} (got: ${id})" >&2
    exit 1
  fi
done

cat > "$OUT_FILE" <<EOF
# Workspace IDs for local Features service testing
# Source this file, then use \$DEPARTMENT_ID and \$TEAM_ID as the
# localResourceId values in the guide's grpcurl examples.
export DEFAULT_ID="${DEFAULT_ID}"
export DEPARTMENT_ID="${DEPARTMENT_ID}"
export TEAM_ID="${TEAM_ID}"
EOF

echo ""
echo "Workspace IDs written to ${OUT_FILE}:"
echo "  DEFAULT_ID=${DEFAULT_ID}"
echo "  DEPARTMENT_ID=${DEPARTMENT_ID}"
echo "  TEAM_ID=${TEAM_ID}"
echo ""
echo "Note: newly created workspaces take ~15 seconds to replicate into"
echo "SpiceDB. Wait briefly before running permission checks against them."
