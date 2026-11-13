#!/bin/sh
# Hermetic wrapper regression tests; run with sh scripts/testcontainers-run_test.sh.
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
wrapper="$script_dir/testcontainers-run.sh"
shell=$(command -v sh)
env_command=$(command -v env)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$tmp/bin" "$tmp/runtime"
# A Linux OS check plus a Podman-only selection avoids all host sockets and CLIs.
printf '#!/bin/sh\nprintf "Linux\\n"\n' > "$tmp/bin/uname"
chmod +x "$tmp/bin/uname"

run_wrapper() {
	"$env_command" -i PATH="$tmp/bin" XDG_RUNTIME_DIR="$tmp/runtime" \
		CONTAINER_RUNTIME=podman "$@"
}

expect_status() {
	expected=$1
	shift
	if output=$(run_wrapper "$@" 2>&1); then
		status=0
	else
		status=$?
	fi
	if [ "$status" -ne "$expected" ]; then
		printf 'Expected status %s, got %s:\n%s\n' "$expected" "$status" "$output" >&2
		exit 1
	fi
}

expect_output() {
	case "$output" in
		*"$1"*) ;;
		*) printf 'Missing output "%s":\n%s\n' "$1" "$output" >&2; exit 1 ;;
	esac
}

expect_status 1 "$shell" "$wrapper" "$shell" -c 'echo command-ran'
expect_output 'No container runtime found'
case "$output" in
	*command-ran*) echo 'Mandatory path ran the command without a runtime' >&2; exit 1 ;;
esac

expect_status 2 "$shell" "$wrapper"
expect_output 'usage:'
expect_status 2 CONTAINER_RUNTIME=invalid "$shell" "$wrapper" "$shell" -c 'exit 0'
expect_output 'Invalid CONTAINER_RUNTIME'

# Explicit endpoints retain normal setup and command arguments/exit status.
expect_status 0 DOCKER_HOST=unix:///controlled/podman.sock "$shell" "$wrapper" "$shell" -c '[ "$DOCKER_HOST" = unix:///controlled/podman.sock ] && [ "$TESTCONTAINERS_RYUK_DISABLED" = true ]'
[ -z "$output" ] || { echo 'Explicit endpoint unexpectedly produced output' >&2; exit 1; }
expect_status 0 DOCKER_HOST=unix:///controlled/podman.sock "$shell" "$wrapper" "$shell" -c 'printf "command-ran: %s\n" "$1"' sh 'argument with spaces'
expect_output 'command-ran: argument with spaces'
expect_status 23 DOCKER_HOST=unix:///controlled/podman.sock "$shell" "$wrapper" "$shell" -c 'exit 23'

printf 'testcontainers-run wrapper regression tests passed\n'
