#!/bin/bash
set -e

source ./scripts/check_docker_podman.sh

COMPOSE_DIR="development/full-kessel"
ENV_FILE="${COMPOSE_DIR}/.env"

${DOCKER} compose --env-file "${ENV_FILE}" \
  --profile relations --profile consumer --profile monitoring \
  -f "${COMPOSE_DIR}/docker-compose.yaml" \
  down
