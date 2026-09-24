# Unified Schema Test Setup

This is a temporary, isolated unified-schema test setup for validating the
Inventory service before unified schema migration is complete. It deliberately
does not modify the shared production schema artifacts or the default Kessel
development stack.

Start it with:

```shell
make kessel-up-unified
```

The setup mounts `resources/` into the Inventory container and uses
`configs/inventory-api.yaml`, which selects `in-memory.type: unified`.

The normal `make kessel-up` flow is unchanged and continues to use the legacy
JSON schema cache.

## Files

- `configs/inventory-api.yaml` selects unified YAML loading for the local stack.
- `resources/*.yaml` contains unified test artifacts for host, workspace, service,
  billing_account, k8s_cluster, k8s_policy, and notifications_integration.
- `docker-compose.override.yaml` mounts the temporary config and resource path.
- `kessel-inventory-ephem-unified.yaml` is a development-only ephemeral template
  that accepts a parameterized unified-schema tarball.

These artifacts are local test fixtures. They are not part of the production
schema set and should not be copied into `data/schema/resources`.

> [!NOTE]
> Some of the test resources (k8s-cluster, k8s-policy, notifications_integration) are there
> for validating schema loading but are not defined in the default schema.zed used in
> this compose setup. If you wish to test those resources, you'll need to create a custom
> schema.zed from the [current stage config](https://raw.githubusercontent.com/project-kessel/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed)
> and add the missing definitions (available in [./deploy/schema.zed](../../deploy/schema.zed)),
> then update the **SCHEMA_ZED_FILE** variable in the full-kessel [.env](../full-kessel/.env) file to use it

## Future Migration

Once unified schema migration is complete, the approved unified schema artifacts
and configuration should move into the common deployment and `kessel-up`
setup. This temporary directory, its compose override, and its ephemeral
template should then be deprecated and removed.

The full-Kessel stack uses the image configured by
`development/full-kessel/.env`. Set `INVENTORY_API_IMAGE` or
`COMPOSE_PULL_MODE=missing` as needed when testing a locally built image.
