#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

COMPOSE_DIR="development/full-kessel"
ENV_FILE="${COMPOSE_DIR}/.env"

# Load .env defaults without overriding caller's environment
if [ -f "${ENV_FILE}" ]; then
  saved_schema_zed_file_set="${SCHEMA_ZED_FILE+x}"
  saved_schema_zed_file="${SCHEMA_ZED_FILE:-}"
  saved_rbac_config_file_set="${RBAC_CONFIG_FILE+x}"
  saved_rbac_config_file="${RBAC_CONFIG_FILE:-}"
  set -a
  source "${ENV_FILE}"
  set +a
  [ -n "${saved_schema_zed_file_set}" ] && SCHEMA_ZED_FILE="${saved_schema_zed_file}"
  [ -n "${saved_rbac_config_file_set}" ] && RBAC_CONFIG_FILE="${saved_rbac_config_file}"
  unset saved_schema_zed_file saved_rbac_config_file saved_schema_zed_file_set saved_rbac_config_file_set
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
  SCHEMA_URL="${SCHEMA_ZED_URL:-https://raw.githubusercontent.com/RedHatInsights/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed}"
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
  RBAC_CONFIG_URL="${RBAC_CONFIG_URL:-https://raw.githubusercontent.com/RedHatInsights/rbac-config/refs/heads/master/_private/configmaps/stage/rbac-config.yml}"
  RBAC_CONFIG_SRC=$(mktemp)
  trap "rm -f ${RBAC_CONFIG_SRC}" EXIT
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
  up --pull "${COMPOSE_PULL_MODE:-always}" -d
