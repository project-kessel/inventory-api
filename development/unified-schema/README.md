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

## Ephemeral Tarball

The ephemeral template expects the unified YAML files in a base64-encoded
tarball passed through the `UNIFIED_RESOURCES_TARBALL` template parameter.

Configure Bonfire to use this local template. Add or update the Kessel Inventory
component in `~/.config/bonfire/config.yaml`:

```yaml
apps:
- name: kessel
  components:
  - name: kessel-inventory
    host: local
    repo: path/to/inventory-api-cloned-repo
    path: development/unified-schema/kessel-inventory-ephem-unified.yaml
```

Build the resource tarball and encode it for the Bonfire parameter override:

```shell
tar -czf /tmp/unified-schema-resources.tar.gz \
  -C development/unified-schema/resources .
UNIFIED_RESOURCES_TARBALL="$(base64 < /tmp/unified-schema-resources.tar.gz | tr -d '\n')"
```

Deploy with Bonfire, overriding the template parameters at deploy time:

```shell
bonfire deploy kessel \
  -C kessel-inventory \
  --local-config-method merge \
  --set-parameter kessel-inventory/INVENTORY_IMAGE=YOU_INVENTORY_IMAGE_REPO \
  --set-parameter kessel-inventory/IMAGE_TAG=YOUR_IMAGE_TAG \
  --set-parameter kessel-inventory/UNIFIED_RESOURCES_TARBALL="${UNIFIED_RESOURCES_TARBALL}"
```

The `--set-parameter` syntax is useful for the tarball because the base64 value
should remain local and should not be committed to the Bonfire config. Other values
can be defined in the config if preferred.

Verify that the ConfigMap contains the expected YAML artifacts:

```shell
oc get configmap resources-tarball \
  -o jsonpath='{.binaryData.resources\.tar\.gz}' \
  | base64 -d \
  | tar -tzf -
```

Verify the Inventory startup log contains the unified repository selection:

```shell
oc logs <inventory-api-pod> | grep 'Using unified YAML in-memory schema repository'
```

## Future Migration

Once unified schema migration is complete, the approved unified schema artifacts
and configuration should move into the common deployment and `kessel-up`
setup. This temporary directory, its compose override, and its ephemeral
template should then be deprecated and removed.

The full-Kessel stack uses the image configured by
`development/full-kessel/.env`. Set `INVENTORY_API_IMAGE` or
`COMPOSE_PULL_MODE=missing` as needed when testing a locally built image.
