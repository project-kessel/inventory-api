# Kessel Inventory API Test Guide

This document contains curl and grpcurl commands to test all Kessel Inventory API endpoints.

**API Base Paths:**
- HTTP: `http://localhost:8000/api/kessel/`
- gRPC: `localhost:9000`

---

## 1. Health Endpoints

### HTTP

#### GET /api/kessel/v1/livez
```bash
curl -s http://localhost:8000/api/kessel/v1/livez
```

**Response:**
```json
{"status":"OK","code":200}
```

#### GET /api/kessel/v1/readyz
```bash
curl -s http://localhost:8000/api/kessel/v1/readyz
```

**Response:**
```json
{"status":"Storage type postgres","code":200}
```

### gRPC

#### GetLivez
```bash
grpcurl -plaintext localhost:9000 kessel.inventory.v1.KesselInventoryHealthService/GetLivez
```

**Response:**
```json
{
  "status": "OK",
  "code": 200
}
```

#### GetReadyz
```bash
grpcurl -plaintext localhost:9000 kessel.inventory.v1.KesselInventoryHealthService/GetReadyz
```

**Response:**
```json
{
  "status": "Storage type postgres",
  "code": 200
}
```

---

## 2. ReportResource

### gRPC

#### Report a Host (HBI)
```bash
grpcurl -plaintext -d '{
  "type": "host",
  "reporterType": "hbi",
  "reporterInstanceId": "3088be62-1c60-4884-b133-9200542d0b3f",
  "representations": {
    "metadata": {
      "localResourceId": "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
      "apiHref": "https://apiHref.com/",
      "consoleHref": "https://www.console.com/",
      "reporterVersion": "2.7.16"
    },
    "common": {
      "workspace_id": "a64d17d0-aec3-410a-acd0-e0b85b22c076"
    },
    "reporter": {
      "subscription_manager_id": "af94f92b-0b65-4cac-b449-6b77e665a08f",
      "insights_id": "05707922-7b0a-4fe6-982d-6adbc7695b8f",
      "ansible_host": "host-1"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource
```

**Response:**
```json
{

}
```

#### Report a K8s Cluster (ACM)
```bash
grpcurl -plaintext -d '{
  "type": "k8s_cluster",
  "reporterType": "ACM",
  "reporterInstanceId": "14c6b63e-49b2-4cc2-99de-5d914b657548",
  "representations": {
    "metadata": {
      "localResourceId": "ae5c7a82-cb3b-4591-9b10-3ae1506d4f3d",
      "apiHref": "https://apiHref.com/",
      "consoleHref": "https://www.console.com/",
      "reporterVersion": "0.2.0"
    },
    "common": {
      "workspace_id": "aee8f698-9d43-49a1-b458-680a7c9dc046"
    },
    "reporter": {
      "external_cluster_id": "9414df93-aefe-4153-ba8a-8765373d39b9",
      "cluster_status": "READY",
      "cluster_reason": "reflect",
      "kube_version": "2.7.0",
      "kube_vendor": "KUBE_VENDOR_UNSPECIFIED",
      "vendor_version": "3.3.1",
      "cloud_platform": "BAREMETAL_IPI",
      "nodes": [
        {
          "name": "www.example.com",
          "cpu": "7500m",
          "memory": "30973224Ki"
        }
      ]
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource
```

**Response:**
```json
{

}
```

#### Report a Notifications Integration
```bash
grpcurl -plaintext -d '{
  "type": "notifications_integration",
  "reporterType": "NOTIFICATIONS",
  "reporterInstanceId": "cc38fb9e-251d-4abe-9eaf-b71607558b2a",
  "representations": {
    "metadata": {
      "localResourceId": "03c923f9-6747-4177-ae35-d36493a1c88e",
      "apiHref": "https://www.example.com/",
      "consoleHref": "http://www.example.net/",
      "reporterVersion": "1.5.7"
    },
    "common": {
      "workspace_id": "1f00e06a-951d-4042-b25b-5ce7c32d833e"
    },
    "reporter": {
      "reporter_type": "NOTIFICATIONS",
      "reporter_instance_id": "f2e4e735-3936-4ee6-a881-b2e1f9326991",
      "local_resource_id": "cbc86170-e959-42d8-bd2a-964a5a558475"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource
```

**Response:**
```json
{

}
```

### HTTP (with x-rh-identity)

```bash
# Set x-rh-identity header
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

# Report a host
curl -X POST http://localhost:8000/api/kessel/v1beta2/resources \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d @data/testData/v1beta2/host.json
```

**Response (403 - meta authorization required):**
```json
{"code":403,"reason":"","message":"meta authorization denied","metadata":{}}
```

---

## 3. DeleteResource

### gRPC

#### Delete a Host
```bash
grpcurl -plaintext -d '{
  "reference": {
    "resourceType": "host",
    "resourceId": "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
    "reporter": {
      "type": "hbi",
      "instanceId": "3088be62-1c60-4884-b133-9200542d0b3f"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/DeleteResource
```

**Response:**
```json
{

}
```

#### Delete a Notifications Integration
```bash
grpcurl -plaintext -d '{
  "reference": {
    "resourceType": "notifications_integration",
    "resourceId": "03c923f9-6747-4177-ae35-d36493a1c88e",
    "reporter": {
      "type": "NOTIFICATIONS",
      "instanceId": "cc38fb9e-251d-4abe-9eaf-b71607558b2a"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/DeleteResource
```

**Response:**
```json
{

}
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X DELETE http://localhost:8000/api/kessel/v1beta2/resources \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "reference": {
      "resource_type": "host",
      "resource_id": "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
      "reporter": {
        "type": "HBI"
      }
    }
  }'
```

---

## 4. Check (Permission Check)

### gRPC

```bash
grpcurl -plaintext -d '{
  "object": {
    "resourceType": "host",
    "resourceId": "test-host-123"
  },
  "relation": "viewer",
  "subject": {
    "resource": {
      "resourceType": "user",
      "resourceId": "user-456"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/Check
```

**Response (validation error - requires proper subject):**
```
ERROR:
  Code: InvalidArgument
  Message: required field is empty
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X POST http://localhost:8000/api/kessel/v1beta2/check \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "object": {
      "resourceType": "host",
      "resourceId": "test-host-123"
    },
    "relation": "viewer",
    "subject": {
      "resource": {
        "resourceType": "user",
        "resourceId": "user-456"
      }
    }
  }'
```

---

## 5. CheckSelf (Self Permission Check)

### gRPC

```bash
grpcurl -plaintext -d '{
  "object": {
    "resourceType": "host",
    "resourceId": "test-host-123"
  },
  "relation": "viewer"
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/CheckSelf
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X POST http://localhost:8000/api/kessel/v1beta2/checkself \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "object": {
      "resourceType": "host",
      "resourceId": "test-host-123"
    },
    "relation": "viewer"
  }'
```

---

## 6. CheckForUpdate (Strongly Consistent Check)

### gRPC

```bash
grpcurl -plaintext -d '{
  "object": {
    "resourceType": "host",
    "resourceId": "test-host-123"
  },
  "relation": "editor",
  "subject": {
    "resource": {
      "resourceType": "user",
      "resourceId": "user-456"
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/CheckForUpdate
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X POST http://localhost:8000/api/kessel/v1beta2/checkforupdate \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "object": {
      "resourceType": "host",
      "resourceId": "test-host-123"
    },
    "relation": "editor",
    "subject": {
      "resource": {
        "resourceType": "user",
        "resourceId": "user-456"
      }
    }
  }'
```

---

## 7. CheckBulk (Bulk Permission Checks)

### gRPC

```bash
grpcurl -plaintext -d '{
  "items": [
    {
      "object": {"resourceType": "host", "resourceId": "host-1"},
      "relation": "viewer",
      "subject": {"resource": {"resourceType": "user", "resourceId": "user-1"}}
    },
    {
      "object": {"resourceType": "host", "resourceId": "host-2"},
      "relation": "editor",
      "subject": {"resource": {"resourceType": "user", "resourceId": "user-1"}}
    }
  ]
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/CheckBulk
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X POST http://localhost:8000/api/kessel/v1beta2/checkbulk \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "items": [
      {
        "object": {"resourceType": "host", "resourceId": "host-1"},
        "relation": "viewer",
        "subject": {"resource": {"resourceType": "user", "resourceId": "user-1"}}
      }
    ]
  }'
```

---

## 8. CheckSelfBulk (Bulk Self Permission Checks)

### gRPC

```bash
grpcurl -plaintext -d '{
  "items": [
    {
      "object": {"resourceType": "host", "resourceId": "host-1"},
      "relation": "viewer"
    },
    {
      "object": {"resourceType": "host", "resourceId": "host-2"},
      "relation": "editor"
    }
  ]
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/CheckSelfBulk
```

### HTTP (with x-rh-identity)

```bash
X_RH_IDENTITY=$(echo -n '{"identity":{"account_number":"12345","org_id":"67890","type":"User","user":{"email":"test@example.com","username":"testuser"}}}' | base64)

curl -X POST http://localhost:8000/api/kessel/v1beta2/checkselfbulk \
  -H "Content-Type: application/json" \
  -H "x-rh-identity: $X_RH_IDENTITY" \
  -d '{
    "items": [
      {"object": {"resourceType": "host", "resourceId": "host-1"}, "relation": "viewer"}
    ]
  }'
```

---

## 9. StreamedListObjects

### gRPC

```bash
grpcurl -plaintext -d '{
  "objectType": {
    "resourceType": "host",
    "reporterType": "hbi"
  },
  "relation": "view",
  "subject": {
    "resource": {
      "resourceType": "principal",
      "resourceId": "sarah",
      "reporter": {
        "type": "rbac"
      }
    }
  }
}' localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/StreamedListObjects
```

**Response:** (empty stream when no matching objects)

---

## 10. List Available gRPC Services

```bash
# List all services
grpcurl -plaintext localhost:9000 list
```

**Response:**
```
grpc.channelz.v1.Channelz
grpc.health.v1.Health
grpc.reflection.v1.ServerReflection
grpc.reflection.v1alpha.ServerReflection
kessel.inventory.v1.KesselInventoryHealthService
kessel.inventory.v1beta2.KesselInventoryService
kratos.api.Metadata
```

```bash
# List KesselInventoryService methods
grpcurl -plaintext localhost:9000 list kessel.inventory.v1beta2.KesselInventoryService
```

**Response:**
```
kessel.inventory.v1beta2.KesselInventoryService.Check
kessel.inventory.v1beta2.KesselInventoryService.CheckBulk
kessel.inventory.v1beta2.KesselInventoryService.CheckForUpdate
kessel.inventory.v1beta2.KesselInventoryService.CheckSelf
kessel.inventory.v1beta2.KesselInventoryService.CheckSelfBulk
kessel.inventory.v1beta2.KesselInventoryService.DeleteResource
kessel.inventory.v1beta2.KesselInventoryService.ReportResource
kessel.inventory.v1beta2.KesselInventoryService.StreamedListObjects
```

```bash
# List KesselInventoryHealthService methods
grpcurl -plaintext localhost:9000 list kessel.inventory.v1.KesselInventoryHealthService
```

**Response:**
```
kessel.inventory.v1.KesselInventoryHealthService.GetLivez
kessel.inventory.v1.KesselInventoryHealthService.GetReadyz
```

---

## Using Test Data Files

The repository includes test data files in `data/testData/v1beta2/`:

```bash
# Report using test data file
grpcurl -plaintext -d "$(cat data/testData/v1beta2/host.json)" \
  localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource

grpcurl -plaintext -d "$(cat data/testData/v1beta2/k8s-cluster.json)" \
  localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource

grpcurl -plaintext -d "$(cat data/testData/v1beta2/notifications-integrations.json)" \
  localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/ReportResource

# Delete using test data file
grpcurl -plaintext -d "$(cat data/testData/v1beta2/delete-host.json)" \
  localhost:9000 kessel.inventory.v1beta2.KesselInventoryService/DeleteResource
```

---

## Notes

1. **HTTP Authentication**: HTTP endpoints require `x-rh-identity` header for authentication
2. **gRPC Authentication**: gRPC endpoints allow unauthenticated access when configured with `allow-unauthenticated: true`
3. **Meta Authorization**: Resource operations (Report/Delete) may require additional meta authorization checks
4. **API Path Prefix**: All endpoints use `/api/kessel/` prefix (changed from `/api/inventory/`)
