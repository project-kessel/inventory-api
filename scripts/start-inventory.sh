#!/bin/bash

export CONFIG=$1
export HTTP_PORT=$2
export GRPC_PORT=$3
shift 3
SERVICES="$@"

set -e
source ./scripts/check_docker_podman.sh
NETWORK_CHECK=$(${DOCKER} network ls --filter name=kessel --format json)
if [[ -z "${NETWORK_CHECK}" || "${NETWORK_CHECK}" == "[]" ]]; then ${DOCKER} network create kessel; fi
${DOCKER} compose -f development/docker-compose.yaml up --build -d ${SERVICES}
