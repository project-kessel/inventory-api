#!/usr/bin/env bash

set -euo pipefail

export ACCESS_CACHE_CONNECT_SIGNALS=False

echo "Starting local RBAC init container script."
python /opt/rbac/rbac/manage.py wait_for_db

echo "Running schema migrations <----"
python /opt/rbac/rbac/manage.py migrate --noinput

if [[ "${REPLICATION_TO_RELATION_ENABLED:-False}" == "True" ]]; then
    echo "Waiting for the RBAC Debezium connector and task to be RUNNING..."
    max_wait=120
    elapsed=0
    while (( elapsed < max_wait )); do
        if python - "${KAFKA_CONNECT_URL:-http://kafka-connect:8083}" "${KAFKA_CONNECTOR_NAME:-rbac-outbox-connector}" <<'PY'
import json
import sys
import urllib.error
import urllib.request

base_url = sys.argv[1].rstrip("/")
connector_name = sys.argv[2]
url = f"{base_url}/connectors/{connector_name}/status"

try:
    with urllib.request.urlopen(url, timeout=5) as response:
        data = json.load(response)
    connector = data.get("connector", {}).get("state", "")
    tasks = data.get("tasks") or []
    task_running = bool(tasks) and all(task.get("state") == "RUNNING" for task in tasks)
    raise SystemExit(0 if connector == "RUNNING" and task_running else 1)
except (urllib.error.URLError, TimeoutError, json.JSONDecodeError, IndexError, KeyError, TypeError, AttributeError):
    raise SystemExit(1)
PY
        then
            echo "RBAC Debezium connector and task are RUNNING."
            break
        fi
        sleep 2
        elapsed=$((elapsed + 2))
    done

    if (( elapsed >= max_wait )); then
        echo "RBAC Debezium connector was not ready after ${max_wait}s."
        exit 1
    fi
fi

echo "Running seeds with forced relation creation <-------"
python /opt/rbac/rbac/manage.py seeds --force-create-relationships
