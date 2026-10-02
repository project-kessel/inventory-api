#!/bin/sh
# Run a command with the same container endpoint that runtime detection probes.
# Usage: ./scripts/testcontainers-run.sh go test ./internal/data/...
# Any Testcontainers-based test command can use this wrapper. Set DOCKER_HOST
# to override detection. For rootless Podman, start the socket first with
# `systemctl --user start podman.socket`; the wrapper disables Ryuk for Podman.
set -eu

if [ "$#" -eq 0 ]; then
	echo "usage: $0 command [args...]" >&2
	exit 2
fi

if [ -z "${DOCKER_HOST:-}" ]; then
	if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
		context=$(docker context show)
		DOCKER_HOST=$(docker context inspect "$context" --format '{{.Endpoints.docker.Host}}')
		if [ -z "$DOCKER_HOST" ]; then
			echo "Docker context $context has no Docker endpoint" >&2
			exit 1
		fi
		export DOCKER_HOST
		echo "Using Docker context $context ($DOCKER_HOST) for Testcontainers"
	elif [ -n "${XDG_RUNTIME_DIR:-}" ] && [ -S "$XDG_RUNTIME_DIR/podman/podman.sock" ]; then
		DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"
		export DOCKER_HOST
		echo "Using rootless Podman for Testcontainers"
	elif [ -S /var/run/docker.sock ]; then
		DOCKER_HOST=unix:///var/run/docker.sock
		export DOCKER_HOST
		echo "Using Docker socket for Testcontainers"
	else
		echo "No container runtime found; start Docker or run: systemctl --user start podman.socket" >&2
		exit 1
	fi
fi

case "$DOCKER_HOST" in
	*podman*) export TESTCONTAINERS_RYUK_DISABLED="${TESTCONTAINERS_RYUK_DISABLED:-true}" ;;
esac

exec "$@"
