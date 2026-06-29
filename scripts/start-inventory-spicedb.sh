#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

export CONFIG=local-w-spicedb
export HTTP_PORT=8000
export GRPC_PORT=9000

SCHEMA_DEST="development/configs/schema.zed"

if [ -n "${SCHEMA_ZED_FILE}" ]; then
  echo "Using local schema file: ${SCHEMA_ZED_FILE}"
  cp "${SCHEMA_ZED_FILE}" "${SCHEMA_DEST}"
else
  SCHEMA_URL="${SCHEMA_ZED_URL:-https://raw.githubusercontent.com/project-kessel/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed}"
  echo "Downloading schema.zed from ${SCHEMA_URL}"
  curl -fsSL -o "${SCHEMA_DEST}" "${SCHEMA_URL}"
fi

NETWORK_CHECK=$(${DOCKER} network ls --filter name=kessel --format json)
if [[ -z "${NETWORK_CHECK}" || "${NETWORK_CHECK}" == "[]" ]]; then ${DOCKER} network create kessel; fi

${DOCKER} compose -f development/docker-compose.yaml -f development/docker-compose.spicedb.yaml up --build -d \
  inventory-api spicedb-database spicedb-migrate spicedb kafka-connect-setup kafka-setup
