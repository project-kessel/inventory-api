#!/bin/bash
set -euo pipefail

source ./scripts/check_docker_podman.sh

COMPOSE_DIR="development/full-kessel"
ENV_FILE="${COMPOSE_DIR}/.env"
DEFAULT_RBAC_OVERRIDE="https://raw.githubusercontent.com/project-kessel/insights-rbac/master/scripts/local_stack/full-kessel.rbac-override.yml"

TEMP_DIR=""
RBAC_CONFIG_SRC=""
RBAC_OVERRIDE_PATH=""
RBAC_INVENTORY_API_CONFIG_DIR="${TMPDIR:-/tmp}/inventory-api-full-kessel"

cleanup() {
  if [[ -n "${TEMP_DIR}" ]]; then
    rm -rf "${TEMP_DIR}"
  fi
}

trap cleanup EXIT

# Load .env defaults without overriding caller's environment
if [ -f "${ENV_FILE}" ]; then
  saved_schema_zed_file_set="${SCHEMA_ZED_FILE+x}"
  saved_schema_zed_file="${SCHEMA_ZED_FILE:-}"
  saved_rbac_config_file_set="${RBAC_CONFIG_FILE+x}"
  saved_rbac_config_file="${RBAC_CONFIG_FILE:-}"
  saved_rbac_override_set="${RBAC_OVERRIDE+x}"
  saved_rbac_override="${RBAC_OVERRIDE:-}"
  set -a
  source "${ENV_FILE}"
  set +a
  [ -n "${saved_schema_zed_file_set}" ] && SCHEMA_ZED_FILE="${saved_schema_zed_file}"
  [ -n "${saved_rbac_config_file_set}" ] && RBAC_CONFIG_FILE="${saved_rbac_config_file}"
  [ -n "${saved_rbac_override_set}" ] && RBAC_OVERRIDE="${saved_rbac_override}"
  unset saved_schema_zed_file saved_rbac_config_file saved_schema_zed_file_set saved_rbac_config_file_set \
    saved_rbac_override saved_rbac_override_set
fi

RBAC_OVERRIDE="${RBAC_OVERRIDE:-${DEFAULT_RBAC_OVERRIDE}}"

TEMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/inventory-api-full-kessel.XXXXXX")"
mkdir -p "${RBAC_INVENTORY_API_CONFIG_DIR}"

# The RBAC compose override enables local Inventory compatibility settings.
# Keep this generated file out of the repository, but retain it after startup
# because the containers may restart and need the bind-mount source to exist.
RBAC_INVENTORY_API_CONFIG="${RBAC_INVENTORY_API_CONFIG_DIR}/inventory-api.yaml"
awk '
  /^authn:$/ {
    print
    print "  allow-unauthenticated: true"
    next
  }
  { print }
' "${COMPOSE_DIR}/configs/inventory-api.yaml" > "${RBAC_INVENTORY_API_CONFIG}"
export RBAC_INVENTORY_API_CONFIG

if [[ "${RBAC_OVERRIDE}" == http://* || "${RBAC_OVERRIDE}" == https://* ]]; then
  RBAC_OVERRIDE_PATH="${TEMP_DIR}/rbac-override.yml"
  echo "Downloading RBAC compose override from ${RBAC_OVERRIDE}"
  curl -fsSL --retry 3 --retry-all-errors -o "${RBAC_OVERRIDE_PATH}" "${RBAC_OVERRIDE}"
else
  if [[ ! -f "${RBAC_OVERRIDE}" ]]; then
    echo "Error: RBAC_OVERRIDE does not exist: ${RBAC_OVERRIDE}"
    exit 1
  fi
  RBAC_OVERRIDE_PATH="${RBAC_OVERRIDE}"
  echo "Using local RBAC compose override: ${RBAC_OVERRIDE_PATH}"
fi

# Check yq is installed (needed to extract RBAC role definitions from configmap YAML)
if ! command -v yq &>/dev/null; then
  echo "Error: yq is required but not installed."
  echo "  Install with:"
  echo "    go install github.com/mikefarah/yq/v4@latest"
  echo "    brew install yq"
  echo "    dnf install yq"
  exit 1
fi

# Create kessel network if it doesn't exist
NETWORK_CHECK=$(${DOCKER} network ls --filter name=kessel --format json)
if [[ -z "${NETWORK_CHECK}" || "${NETWORK_CHECK}" == "[]" ]]; then
  ${DOCKER} network create kessel
fi

# Fetch or copy schema.zed for SpiceDB
SCHEMA_DEST="${COMPOSE_DIR}/configs/schema.zed"
if [ -n "${SCHEMA_ZED_FILE}" ]; then
  echo "Using local schema file: ${SCHEMA_ZED_FILE}"
  cp "${SCHEMA_ZED_FILE}" "${SCHEMA_DEST}"
else
  SCHEMA_URL="${SCHEMA_ZED_URL:-https://raw.githubusercontent.com/project-kessel/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed}"
  echo "Downloading schema.zed from ${SCHEMA_URL}"
  curl -fsSL -o "${SCHEMA_DEST}" "${SCHEMA_URL}"
fi

# Fetch RBAC role definitions from stage configmap (replaces baked-in definitions
# that include poisoned approval_* roles not in the SpiceDB schema)
RBAC_DEFS_DIR="${COMPOSE_DIR}/configs/rbac-role-definitions"
mkdir -p "${RBAC_DEFS_DIR}"
rm -f "${RBAC_DEFS_DIR}"/*.json 2>/dev/null
if [ -n "${RBAC_CONFIG_FILE}" ]; then
  echo "Using local RBAC config: ${RBAC_CONFIG_FILE}"
  RBAC_CONFIG_SRC="${RBAC_CONFIG_FILE}"
else
  RBAC_CONFIG_URL="${RBAC_CONFIG_URL:-https://raw.githubusercontent.com/project-kessel/rbac-config/refs/heads/master/_private/configmaps/stage/rbac-config.yml}"
  RBAC_CONFIG_SRC="${TEMP_DIR}/rbac-config.yml"
  echo "Downloading RBAC role definitions from ${RBAC_CONFIG_URL}"
  curl -fsSL -o "${RBAC_CONFIG_SRC}" "${RBAC_CONFIG_URL}"
fi
for key in $(yq '.objects[0].data | keys | .[]' "${RBAC_CONFIG_SRC}"); do
  yq -r ".objects[0].data[\"${key}\"]" "${RBAC_CONFIG_SRC}" > "${RBAC_DEFS_DIR}/${key}"
done
echo "Extracted $(ls "${RBAC_DEFS_DIR}"/*.json 2>/dev/null | wc -l) RBAC role definition files"

${DOCKER} compose --env-file "${ENV_FILE}" \
  --profile relations --profile consumer --profile rbac "$@" \
  -f "${COMPOSE_DIR}/docker-compose.yaml" \
  -f "${RBAC_OVERRIDE_PATH}" \
  up --pull "${COMPOSE_PULL_MODE:-always}" -d
