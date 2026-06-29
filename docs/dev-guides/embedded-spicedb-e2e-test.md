# Local SpiceDB Compose Runbook

Validates the embedded SpiceDB authz pipeline: direct tuple management, permission checks, and the consumer-driven resource flow through Kafka to SpiceDB.

## 1. Start the stack

```bash
make inventory-up-spicedb
```

Builds inventory-api from source and starts it with SpiceDB, Kafka, and the inventory consumer.

## 2. Verify services are healthy

```bash
curl -s http://localhost:8000/api/kessel/v1/livez
podman ps | grep development-
```

Should show `{"status":"OK", "code":200}` and containers for inventory-api, invdatabase, spicedb, spicedb-database, kafka, zookeeper, and kafka-connect.

## 3. Write the schema to SpiceDB

```bash
zed schema write development/configs/schema.zed --endpoint localhost:50051 --token foobar --insecure
```

The schema file is downloaded automatically during step 1. This loads it into SpiceDB.

## 4. Create the permission chain via CreateTuples

```bash
grpcurl -plaintext -d '{
  "upsert": true,
  "tuples": [
    {
      "resource": {"type": {"namespace": "rbac", "name": "role"}, "id": "test-viewer-role"},
      "relation": "t_inventory_hosts_read",
      "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "*"}}
    },
    {
      "resource": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "test-rb-1"},
      "relation": "t_role",
      "subject": {"subject": {"type": {"namespace": "rbac", "name": "role"}, "id": "test-viewer-role"}}
    },
    {
      "resource": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "test-rb-1"},
      "relation": "t_subject",
      "subject": {"subject": {"type": {"namespace": "rbac", "name": "principal"}, "id": "test-user-1"}}
    },
    {
      "resource": {"type": {"namespace": "rbac", "name": "workspace"}, "id": "test-ws-1"},
      "relation": "t_binding",
      "subject": {"subject": {"type": {"namespace": "rbac", "name": "role_binding"}, "id": "test-rb-1"}}
    },
    {
      "resource": {"type": {"namespace": "hbi", "name": "host"}, "id": "test-host-1"},
      "relation": "t_workspace",
      "subject": {"subject": {"type": {"namespace": "rbac", "name": "workspace"}, "id": "test-ws-1"}}
    }
  ]
}' localhost:9000 kessel.inventory.v1beta2.KesselTupleService/CreateTuples
```

Sets up a role granting `t_inventory_hosts_read`, binds a user to it, assigns the binding to a workspace, and places a host in that workspace.

## 5. Verify tuples landed in SpiceDB

```bash
zed relationship read hbi/host --endpoint localhost:50051 --token foobar --insecure
zed relationship read rbac/role_binding --endpoint localhost:50051 --token foobar --insecure
```

Confirms the tuples are stored. Note that `zed` outputs to stderr, so pipe with `2>&1` if redirecting.

## 6. Check permission -- authorized user

```bash
grpcurl -plaintext -d '{
  "object": {"resource_type": "host", "resource_id": "test-host-1", "reporter": {"type": "hbi"}},
  "relation": "view",
  "subject": {"resource": {"resource_type": "principal", "resource_id": "test-user-1", "reporter": {"type": "rbac"}}}
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/Check
```

Should return `ALLOWED_TRUE`.

## 7. Check permission -- unauthorized user

```bash
grpcurl -plaintext -d '{
  "object": {"resource_type": "host", "resource_id": "test-host-1", "reporter": {"type": "hbi"}},
  "relation": "view",
  "subject": {"resource": {"resource_type": "principal", "resource_id": "unauthorized-user", "reporter": {"type": "rbac"}}}
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/Check
```

Should return `ALLOWED_FALSE`.

## 8. Report a resource and verify consumer pipeline

```bash
grpcurl -plaintext -d '{
  "type": "host",
  "reporter_type": "hbi",
  "reporter_instance_id": "hbi-instance-01",
  "representations": {
    "metadata": {
      "local_resource_id": "consumer-test-host-001",
      "api_href": "/api/inventory/v1/hosts/consumer-test-host-001"
    },
    "common": {
      "workspace_id": "test-ws-1"
    },
    "reporter": {
      "fqdn": "consumer-test.example.com"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService.ReportResource
```

Creates a resource through the normal API path. The inventory consumer picks up the outbox event from Kafka and writes the workspace tuple to SpiceDB automatically. Verify with:

```bash
zed relationship read hbi/host --endpoint localhost:50051 --token foobar --insecure
```

You should see a `t_workspace` relationship for the new host pointing at `test-ws-1`.

## 9. Tear down

```bash
make inventory-down
```
