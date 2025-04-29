#!/bin/bash
set -e

export CONFIG=""
export HTTP_PORT=1
export GRPC_PORT=1

# Function to check if a command is available
source ./scripts/check_docker_podman.sh
${DOCKER} compose -f development/docker-compose.yaml down
