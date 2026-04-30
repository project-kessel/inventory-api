#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

COMPOSE_DIR="development/full-kessel"
ENV_FILE="${COMPOSE_DIR}/.env"

# Load .env for schema handling
if [ -f "${ENV_FILE}" ]; then
  set -a
  source "${ENV_FILE}"
  set +a
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

${DOCKER} compose --env-file "${ENV_FILE}" \
  --profile relations --profile consumer --profile rbac "$@" \
  -f "${COMPOSE_DIR}/docker-compose.yaml" \
  up -d
