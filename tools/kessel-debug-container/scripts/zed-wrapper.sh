#!/bin/bash

# Zed wrapper script to deter 'write' and 'delete' operations while allowing all other commands
# This provides a security policy layer for debug/read-only containers

# Check if the any arguments provided are for write operations
if [[ "$@" =~ "write" ]] || [[ "$@" =~ "delete" ]]; then
echo "ERROR: write and delete operations are disabled in this container for security reasons." >&2
echo "This container is configured for read-only operations only." >&2
exit 1
fi

# For all other commands, pass through to the original zed binary
exec /usr/bin/zed.original "$@"
