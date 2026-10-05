#!/bin/sh
# Run a command with the same container endpoint that runtime detection probes.
# Usage: ./scripts/testcontainers-run.sh go test ./internal/data/...
# Any Testcontainers-based test command can use this wrapper. Set DOCKER_HOST
# to override detection. Ensure Docker or Podman is running and its API socket
# is available; the wrapper disables Ryuk for Podman.
# Set CONTAINER_RUNTIME=docker or podman to restrict automatic detection.
set -eu

if [ "$#" -eq 0 ]; then
	echo "usage: $0 command [args...]" >&2
	exit 2
fi

container_runtime=${CONTAINER_RUNTIME:-auto}
case "$container_runtime" in
	auto|docker|podman) ;;
	*) echo "Invalid CONTAINER_RUNTIME: $container_runtime; expected auto, docker, or podman." >&2; exit 2 ;;
esac

if [ -z "${DOCKER_HOST:-}" ]; then
	if [ "$container_runtime" != podman ] && command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
		context=$(docker context show)
		DOCKER_HOST=$(docker context inspect "$context" --format '{{.Endpoints.docker.Host}}')
		if [ -z "$DOCKER_HOST" ]; then
			echo "Docker context $context has no Docker endpoint" >&2
			exit 1
		fi
		export DOCKER_HOST
		echo "Using Docker context $context ($DOCKER_HOST) for Testcontainers"
	elif [ "$container_runtime" != docker ] && [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -S "$XDG_RUNTIME_DIR/podman/podman.sock" ]; then
		DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
		export DOCKER_HOST
		echo "Using rootless Podman for Testcontainers"
	elif [ "$container_runtime" != docker ] && [ "$(uname -s)" = Darwin ] && command -v podman >/dev/null 2>&1 && podman info >/dev/null 2>&1; then
		podman_socket=$(podman machine inspect --format '{{.ConnectionInfo.PodmanSocket.Path}}' 2>/dev/null) || podman_socket=
		if [ -z "$podman_socket" ] || [ ! -S "$podman_socket" ]; then
			echo "Podman machine API socket unavailable; ensure your Podman machine is running properly, or set DOCKER_HOST to its endpoint." >&2
			exit 1
		fi
		DOCKER_HOST="unix://$podman_socket"
		export DOCKER_HOST
		echo "Using Podman machine for Testcontainers ($DOCKER_HOST)"
	elif [ "$container_runtime" != podman ] && [ -S /var/run/docker.sock ]; then
		DOCKER_HOST=unix:///var/run/docker.sock
		export DOCKER_HOST
		echo "Using Docker socket for Testcontainers"
	else
		echo "No container runtime found (selection: $container_runtime); ensure the selected runtime is installed, running properly, and its API socket is available. Set DOCKER_HOST to select a specific endpoint." >&2
		exit 1
	fi
fi

case "$DOCKER_HOST" in
	*podman*) export TESTCONTAINERS_RYUK_DISABLED="${TESTCONTAINERS_RYUK_DISABLED:-true}" ;;
esac

exec "$@"
