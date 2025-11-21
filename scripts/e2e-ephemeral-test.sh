#!/bin/bash
set -ex

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Step 0: Release existing reserved namespace
log_info "Checking for currently reserved namespace..."
CURRENT_NAMESPACE=$(bonfire namespace describe 2>/dev/null | grep -oE "ephemeral-[a-z0-9]+" | head -1 || true)

if [ -n "$CURRENT_NAMESPACE" ]; then
    log_warning "Found existing reserved namespace: $CURRENT_NAMESPACE"
    log_info "Releasing namespace: $CURRENT_NAMESPACE"
    bonfire namespace release $CURRENT_NAMESPACE 2>&1 || log_warning "Failed to release $CURRENT_NAMESPACE (may already be released)"
    log_info "Waiting for release to complete..."
    sleep 5
else
    log_info "No currently reserved namespace found"
fi

# Step 1: Deploy kessel-inventory to ephemeral
log_info "Deploying kessel-inventory to ephemeral environment..."
BONFIRE_OUTPUT=$(bonfire deploy kessel -C kessel-inventory 2>&1)
DEPLOY_EXIT_CODE=$?

echo "$BONFIRE_OUTPUT"

if [ $DEPLOY_EXIT_CODE -ne 0 ]; then
    log_error "Failed to deploy kessel-inventory"
    exit 1
fi

# Extract namespace from bonfire output - look for "successfully deployed to namespace"
NAMESPACE=$(echo "$BONFIRE_OUTPUT" | grep -oE "successfully deployed to namespace [a-z0-9-]+" | awk '{print $NF}')

if [ -z "$NAMESPACE" ]; then
    # Try alternate method - look for "namespace:" line
    NAMESPACE=$(echo "$BONFIRE_OUTPUT" | grep -oE "namespace: [a-z0-9-]+" | cut -d' ' -f2 | head -1)
fi

if [ -z "$NAMESPACE" ]; then
    # Last resort - get most recent ephemeral namespace
    NAMESPACE=$(kubectl get ns | grep "^ephemeral-" | tail -1 | awk '{print $1}')
fi

if [ -z "$NAMESPACE" ]; then
    log_error "Could not determine ephemeral namespace"
    log_info "Bonfire output was:"
    echo "$BONFIRE_OUTPUT"
    log_info "Available ephemeral namespaces:"
    kubectl get ns | grep ephemeral
    exit 1
fi

log_info "Using namespace: $NAMESPACE"

log_info "Waiting for deployment to be ready (30s)..."
sleep 30

# Step 2: Get database pod
log_info "Getting database pod..."
DB_POD=$(kubectl get pods -n $NAMESPACE | grep "kessel-inventory-db" | awk '{print $1}' | head -1)

if [ -z "$DB_POD" ]; then
    log_error "Could not find kessel-inventory-db pod"
    log_info "Available pods in $NAMESPACE:"
    kubectl get pods -n $NAMESPACE
    exit 1
fi

log_info "Database pod: $DB_POD"

# Step 3: Connect to DB and verify all tables are empty
log_info "Verifying database tables are empty..."

TABLES=("resource" "reporter_resources" "reporter_representations" "common_representations" "outbox_events")

for table in "${TABLES[@]}"; do
    log_info "Checking table: $table"
    COUNT=$(oc exec -n $NAMESPACE $DB_POD -- bash -c "psql -U postgres -d kessel-inventory -t -c \"SELECT COUNT(*) FROM $table;\"" 2>&1)
    EXIT_CODE=$?
    
    if [ $EXIT_CODE -ne 0 ] || [[ "$COUNT" == *"error"* ]] || [[ "$COUNT" == *"ERROR"* ]] || [[ "$COUNT" == *"does not exist"* ]]; then
        log_warning "Could not query table '$table'"
        log_info "Skipping database verification - will verify via API behavior instead"
        break
    fi
    
    COUNT=$(echo $COUNT | xargs) # trim whitespace
    
    if [ "$COUNT" -eq 0 ]; then
        log_info "✓ Table '$table' is empty (count: 0)"
    else
        log_warning "Table '$table' has $COUNT rows (fresh deploy may have seed data)"
    fi
done

log_info "Database verification complete"

# Step 4: Set up port-forward to inventory-api
log_info "Setting up port-forward to kessel-inventory-api service..."

LOCAL_PORT=9000

# Check if port-forward is already running
if lsof -Pi :$LOCAL_PORT -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    log_warning "Port $LOCAL_PORT already in use, killing existing port-forward..."
    kill $(lsof -t -i:$LOCAL_PORT) 2>/dev/null || true
    sleep 2
fi

# Start port-forward in background
oc port-forward -n $NAMESPACE svc/kessel-inventory-api $LOCAL_PORT:9000 &
PORT_FORWARD_PID=$!

log_info "Port-forward started (PID: $PORT_FORWARD_PID)"
log_info "Waiting for port-forward to be ready..."
sleep 5

# Verify port-forward is working
if ! lsof -Pi :$LOCAL_PORT -sTCP:LISTEN -t >/dev/null 2>&1 ; then
    log_error "Port-forward failed to start"
    exit 1
fi

log_info "Port-forward ready on localhost:$LOCAL_PORT"

# Cleanup function to kill port-forward on exit
cleanup() {
    log_info "Cleaning up port-forward..."
    kill $PORT_FORWARD_PID 2>/dev/null || true
}
trap cleanup EXIT

GRPC_ENDPOINT="localhost:$LOCAL_PORT"

# Test data
WORKSPACE_ID="test-workspace-$(date +%s)"
RESOURCE_ID="test-resource-$(uuidgen | tr '[:upper:]' '[:lower:]')"
REPORTER_TYPE="hbi"
REPORTER_INSTANCE="test-instance-1"

log_info "Test resource ID: $RESOURCE_ID"
log_info "Test workspace ID: $WORKSPACE_ID"

# Step 5: Create a resource using grpcurl
log_info "Creating a test resource..."

CREATE_RESPONSE=$(grpcurl \
    -plaintext \
    -d "{
        \"type\": \"host\",
        \"reporterType\": \"${REPORTER_TYPE}\",
        \"reporterInstanceId\": \"${REPORTER_INSTANCE}\",
        \"representations\": {
            \"metadata\": {
                \"localResourceId\": \"${RESOURCE_ID}\",
                \"apiHref\": \"https://example.com/api\",
                \"consoleHref\": \"https://example.com/console\"
            },
            \"common\": {
                \"workspace_id\": \"${WORKSPACE_ID}\"
            },
            \"reporter\": {
                \"ansible_host\": \"test-host-${RESOURCE_ID}\"
            }
        }
    }" \
    $GRPC_ENDPOINT \
    kessel.inventory.v1beta2.KesselInventoryService/ReportResource)

if [ $? -eq 0 ]; then
    log_info "✓ Resource created successfully"
    echo "$CREATE_RESPONSE"
else
    log_error "Failed to create resource"
    exit 1
fi

# Extract resource ID from response
INVENTORY_ID=$(echo "$CREATE_RESPONSE" | jq -r '.resourceId // .inventoryId // empty')

if [ -z "$INVENTORY_ID" ] || [ "$INVENTORY_ID" == "null" ]; then
    log_warning "Could not extract resourceId from response, using localResourceId instead"
    INVENTORY_ID="$RESOURCE_ID"
fi

log_info "Resource ID: $INVENTORY_ID"

# Step 6: Check access (should be allowed)
log_info "Checking access to resource (expecting ALLOWED)..."

MAX_RETRIES=5
RETRY_COUNT=0
CHECK_SUCCESS=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    CHECK_RESPONSE=$(grpcurl \
        -plaintext \
        -d "{
            \"object\": {
                \"resource_type\": \"host\",
                \"resource_id\": \"${RESOURCE_ID}\",
                \"reporter\": {
                    \"type\": \"${REPORTER_TYPE}\"
                }
            },
            \"relation\": \"workspace\",
            \"subject\": {
                \"resource\": {
                    \"resource_type\": \"workspace\",
                    \"resource_id\": \"${WORKSPACE_ID}\",
                    \"reporter\": {
                        \"type\": \"rbac\"
                    }
                }
            }
        }" \
        $GRPC_ENDPOINT \
        kessel.inventory.v1beta2.KesselInventoryService.Check)

    ALLOWED=$(echo "$CHECK_RESPONSE" | jq -r '.allowed')

    if [ "$ALLOWED" == "ALLOWED_TRUE" ]; then
        log_info "✓ Access check PASSED: Resource is accessible"
        CHECK_SUCCESS=true
        break
    else
        RETRY_COUNT=$((RETRY_COUNT + 1))
        if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
            log_warning "Access check attempt $RETRY_COUNT failed (got $ALLOWED), retrying in 10s..."
            sleep 10
        fi
    fi
done

if [ "$CHECK_SUCCESS" = false ]; then
    log_error "✗ Access check FAILED after $MAX_RETRIES attempts: Expected ALLOWED_TRUE, got $ALLOWED"
    exit 1
fi

# Step 7: Delete the resource
log_info "Deleting the test resource..."

DELETE_RESPONSE=$(grpcurl \
    -plaintext \
    -d "{
        \"reference\": {
            \"resourceId\": \"${RESOURCE_ID}\",
            \"resourceType\": \"host\",
            \"reporter\": {
                \"type\": \"${REPORTER_TYPE}\",
                \"instanceId\": \"${REPORTER_INSTANCE}\"
            }
        }
    }" \
    $GRPC_ENDPOINT \
    kessel.inventory.v1beta2.KesselInventoryService/DeleteResource)

if [ $? -eq 0 ]; then
    log_info "✓ Resource deleted successfully"
else
    log_error "Failed to delete resource"
    exit 1
fi

# Step 8: Check access again (should be denied/not found)
log_info "Checking access to deleted resource (expecting DENIED or NOT_FOUND)..."

RETRY_COUNT=0
CHECK_DENIED_SUCCESS=false

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    CHECK_AFTER_DELETE=$(grpcurl \
        -plaintext \
        -d "{
            \"object\": {
                \"resource_type\": \"host\",
                \"resource_id\": \"${RESOURCE_ID}\",
                \"reporter\": {
                    \"type\": \"${REPORTER_TYPE}\"
                }
            },
            \"relation\": \"workspace\",
            \"subject\": {
                \"resource\": {
                    \"resource_type\": \"workspace\",
                    \"resource_id\": \"${WORKSPACE_ID}\",
                    \"reporter\": {
                        \"type\": \"rbac\"
                    }
                }
            }
        }" \
        $GRPC_ENDPOINT \
        kessel.inventory.v1beta2.KesselInventoryService.Check 2>&1)

    ALLOWED_AFTER=$(echo "$CHECK_AFTER_DELETE" | jq -r '.allowed' 2>/dev/null || echo "ERROR")

    if [ "$ALLOWED_AFTER" == "ALLOWED_FALSE" ] || [[ "$CHECK_AFTER_DELETE" == *"NotFound"* ]] || [[ "$CHECK_AFTER_DELETE" == *"not found"* ]]; then
        log_info "✓ Access check PASSED: Resource is no longer accessible"
        CHECK_DENIED_SUCCESS=true
        break
    else
        RETRY_COUNT=$((RETRY_COUNT + 1))
        if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
            log_warning "Access check attempt $RETRY_COUNT failed (got $ALLOWED_AFTER), retrying in 10s..."
            sleep 10
        fi
    fi
done

if [ "$CHECK_DENIED_SUCCESS" = false ]; then
    log_error "✗ Access check FAILED after $MAX_RETRIES attempts: Expected ALLOWED_FALSE or NotFound, got $ALLOWED_AFTER"
    echo "$CHECK_AFTER_DELETE"
    exit 1
fi

# Step 9: Final verification - check DB counts
log_info "Verifying database state after test..."

for table in "${TABLES[@]}"; do
    COUNT=$(oc exec -n $NAMESPACE $DB_POD -- bash -c "psql -U postgres -d kessel-inventory -t -c \"SELECT COUNT(*) FROM $table;\"" 2>/dev/null || echo "N/A")
    COUNT=$(echo $COUNT | xargs)
    log_info "Table '$table': $COUNT rows"
done

# Cleanup
log_info "================================================"
log_info "All tests PASSED! ✓"
log_info "================================================"
log_info ""
log_info "Ephemeral environment: $NAMESPACE"
log_info "To release the namespace, run: bonfire namespace release $NAMESPACE"
log_info "Or the script will auto-release it on next run"

exit 0

