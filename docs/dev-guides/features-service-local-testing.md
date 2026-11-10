# Features Service Local Testing Guide

This guide walks the Features service team through running Kessel locally and exercising the inventory API against the Features data model: reporting billing accounts and workspaces, adding and removing service relationships, checking whether a workspace has a service enabled, and deleting resources.

Every command is copy-pasteable. No prior Kessel knowledge is assumed.

## Prerequisites

- `podman` (or Docker) with the compose plugin
- A clone of this repository
- [`grpcurl`](https://github.com/fullstorydev/grpcurl) - used for all inventory API calls
- `curl`, `jq` - used by the workspace setup script

The examples below talk to two local ports:

| Port | Service |
|------|---------|
| 9081 | Inventory API (gRPC) |
| 9080 | RBAC service (HTTP - used only by the setup script) |

## Start the Kessel stack

From the repository root:

```shell
make kessel-up
```

This starts the full local stack (Inventory API, SpiceDB, the Inventory Consumer, Kafka, and the RBAC service) using pre-built images. The command waits until every service is healthy; when it
returns, the stack is ready. Startup takes a few minutes the first time while images are pulled.

To stop everything later (and wipe all local data for a clean slate):

```shell
make kessel-down
```

## The Features model in 60 seconds (for those needing some background)

Three concepts:

- **Workspaces** form a tree: an org has departments, departments have teams. Every workspace inherits from its ancestors.
- **Billing accounts** pay for services.
- **Workspaces** desire services (service preferences).

A workspace has a service **enabled** only when *both* sides hold, and each side is inherited independently down the workspace tree:

```
workspace has service enabled  =  (workspace or an ancestor desires it)
                               AND (a billing account linked to the
                                    workspace or an ancestor pays for it)
```

So a team can desire a service that a department-level account pays for; desire and payment can live at different levels of the tree.

The API surface used in this guide:

| RPC | What it does |
|-----|--------------|
| `ReportResource` | Writes a **full snapshot** of a resource;  every call replaces the previous state |
| `AddResourceRelationships` | Adds only the named relationship values; everything else is untouched |
| `RemoveResourceRelationships` | Removes only the named relationship values |
| `Check` | Asks the permission engine (SpiceDB) whether a relation holds |
| `DeleteResource` | Soft-deletes a resource |

> [!NOTE]
> One thing to internalize early: **writes are asynchronous**. Every write is replicated to the permission engine through a consumer pipeline. Generally replication is very fast, especially with no traffic. That said, waiting a few seconds before issuing a `Check` ensures ample time for replication. If a `Check` looks wrong right after a write, wait and retry.

## One-time workspace setup

The workspace hierarchy itself (which workspace is a child of which) is managed by the RBAC service, not by the inventory API. A setup script handles all of that for you:

```shell
./scripts/features-workspace-setup.sh
```

The script:

1. Bootstraps your local tenant (creating the root and default workspaces)
2. Creates a **Features Department** workspace under default, and a
   **Features Team** workspace under the department
3. Writes the workspace IDs to `./features-test-workspaces.env`

The resulting hierarchy looks like this:

```
            root
              |
           default
              |
      Features Department
              |
        Features Team
```

Later in this guide, the billing account link is reported on the department and the service preference on the team, so the team's access relies on inheriting the payment from its parent.

It is safe to re-run - existing workspaces are reused rather than duplicated.

Then load the IDs into your shell. All later examples use them:

```shell
source ./features-test-workspaces.env
```

> Newly created workspaces may take a few seconds to replicate into the permission engine, same as inventory writes.

## Report a billing account and workspaces

`ReportResource` creates or replaces a resource's full state. Three calls set up the scenario used for the rest of this guide: an account that pays for one service, a department linked to that account, and a team that desires the service.

**1. A billing account that pays for a service** (the service is identified by its UUID - use any UUID while testing locally):

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "representations": {
      "metadata": {
        "localResourceId": "22222222-2222-2222-2222-222222222222",
        "apiHref": "https://api.example.com/billing/22222222",
        "consoleHref": "https://console.example.com/billing/22222222",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "services": ["11111111-1111-1111-1111-111111111111"]
      }
    }
}
EOF
```

**2. The department, linked to the billing account** (payment lives on the department, so every workspace beneath it inherits it). Note the `localResourceId` is the department's workspace ID from the setup script:

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "representations": {
      "metadata": {
        "localResourceId": "$DEPARTMENT_ID",
        "apiHref": "https://api.example.com/workspaces/department",
        "consoleHref": "https://console.example.com/workspaces/department",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_billing_account": "22222222-2222-2222-2222-222222222222"
      }
    }
}
EOF
```

**3. The team, desiring the service**:

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "representations": {
      "metadata": {
        "localResourceId": "$TEAM_ID",
        "apiHref": "https://api.example.com/workspaces/team",
        "consoleHref": "https://console.example.com/workspaces/team",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "direct_service_preferences": ["11111111-1111-1111-1111-111111111111"]
      }
    }
}
EOF
```

**Variant: desiring every service.** Instead of listing services one by one, the team can set `desire_all_services` to true and skip the preferences list entirely. The team then desires every service, so its enabled services become exactly the services its reachable billing accounts pay for:

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.ReportResource <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "representations": {
      "metadata": {
        "localResourceId": "$TEAM_ID",
        "apiHref": "https://api.example.com/workspaces/team",
        "consoleHref": "https://console.example.com/workspaces/team",
        "reporterVersion": "1.0.0"
      },
      "reporter": {
        "desire_all_services": true
      }
    }
}
EOF
```

With this variant the `Check` in the next section returns `ALLOWED_TRUE` for any service the billing account pays for, without naming them in preferences. Desire without payment still denies: a service nobody pays for returns `ALLOWED_FALSE`. Both fields can also be combined in one report - `desire_all_services` widens the desire, `direct_service_preferences` adds specific entries on top.

Each call should return `{}` (an empty success response). A few field notes:

- `localResourceId` is *your* identifier for the resource, scoped to the reporter. For workspaces, use the RBAC workspace IDs so the permission model can see the hierarchy
- `apiHref` is required (the request is rejected without it)
  `consoleHref` and `reporterVersion` are good practice
- `reporterInstanceId` identifies your service instance - any stable string works locally

## Check whether a workspace has a service enabled

Wait a few seconds for the reports to replicate, then Check the permissions. The team desires the service and the department's account pays for it, so this should be allowed:

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.Check <<EOF
{
  "object": {
    "resourceType": "workspace",
    "resourceId": "$TEAM_ID",
    "reporter": { "type": "features" }
  },
  "relation": "enabled_services",
  "subject": {
    "resource": {
      "resourceType": "service",
      "resourceId": "11111111-1111-1111-1111-111111111111",
      "reporter": { "type": "features" }
    }
  }
}
EOF
```

Expected response:

```json
{
  "allowed": "ALLOWED_TRUE"
}
```

(`` `"allowed": "ALLOWED_FALSE"` `` means the permission does not hold.)

Try a service nobody pays for - swap in a random UUID as the subject's `resourceId` and the same call returns `ALLOWED_FALSE`

## Add relationships without re-reporting

`AddResourceRelationships` changes only the named values. To enable a second service for the team, both sides need to move: the account must pay for it and the team must desire it.

**Add the service to the account's paid services:**

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
    "type": "billing_account",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "localResourceId": "22222222-2222-2222-2222-222222222222",
    "reporter": {
      "services": ["33333333-3333-3333-3333-333333333333"]
    }
}
EOF
```

**Add the preference to the team:**

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService/AddResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "localResourceId": "$TEAM_ID",
    "reporter": {
      "direct_service_preferences": ["33333333-3333-3333-3333-333333333333"]
    }
}
EOF
```

Wait a few more seconds, then re-run the `Check` from the previous section with the new service UUID as the subject - it now returns `ALLOWED_TRUE`.

Things worth knowing:

- **Add merges, it does not replace.** The account still pays for the first service; only the named values changed.
- The request names the resource directly (`localResourceId` at the top level) rather than through a `representations` block.
- These endpoints are for relationship fields only (`services`, `direct_billing_account`, `direct_service_preferences`). On this local stack a field that is not in the schema is rejected with `INVALID_ARGUMENT`; a field that exists in the schema but is not a relationship is accepted silently but has no effect on permissions. Either way, stick to the relationship fields.
- Calling `AddResourceRelationships` for a resource that doesn't exist yet will create the resource as well.

## Remove relationships

`RemoveResourceRelationships` is the mirror of Add;  it removes only the named values and leaves everything else untouched:

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService/RemoveResourceRelationships <<EOF
{
    "type": "workspace",
    "reporterType": "features",
    "reporterInstanceId": "features-local-testing",
    "localResourceId": "$TEAM_ID",
    "reporter": {
      "direct_service_preferences": ["33333333-3333-3333-3333-333333333333"]
    }
}
EOF
```

The account still pays for the service, but the team no longer desires it. After a few seconds the `Check` for the second service returns `ALLOWED_FALSE`. Removing a value that is not present is a harmless no-op.

## Delete a resource

`DeleteResource` takes a reference to the resource (the `resourceId` is the `localResourceId` you reported it under):

```shell
grpcurl -plaintext -d @ localhost:9081 kessel.inventory.v1beta2.KesselInventoryService.DeleteResource <<EOF
{
  "reference": {
    "resourceType": "billing_account",
    "resourceId": "22222222-2222-2222-2222-222222222222",
    "reporter": {
      "type": "features",
      "instanceId": "features-local-testing"
    }
  }
}
EOF
```

After replication, the billing account's relationships are cleaned up and checks that depended on its payment return `ALLOWED_FALSE`. The delete is a soft delete: re-reporting the same resource revives it.

## Troubleshooting

- **A `Check` result looks stale** - writes replicate asynchronously. Wait and retry before debugging further.
- **`INVALID_ARGUMENT ... api_href: must be at least 1 characters`** - `apiHref` is required on `ReportResource`; the examples above include it.
- **`INVALID_ARGUMENT ... Additional property ... is not allowed`** - the field is not part of the resource's schema at all. The Add/Remove endpoints are for relationship fields (`services`, `direct_billing_account`, `direct_service_preferences`); note that a schema field that is not a relationship (e.g. `desire_all_services`) is accepted on this local stack but does not affect permissions.
- **Workspaces are not linked to each other** - the hierarchy comes from the setup script (via RBAC). Inventory-reported workspaces have no parent link unless the workspace was created through the script or RBAC.
- **Need a clean slate** - `make kessel-down` stops the stack and removes all local data; `make kessel-up` then starts fresh.
- **Stack logs** - `podman logs -f full-kessel-inventory-api-1` and `podman logs -f full-kessel-inventory-consumer-1` are the usual suspects.

## Going further

The test script `development/unified-schema/features-service-schema-tester.sh` runs a similar automated version of this entire walkthrough (report → check → add → remove, with assertions) against the unified-schema stack (`make kessel-up-unified`). It is a good next step once the manual flow above feels comfortable.
