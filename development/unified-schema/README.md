# Unified Schema Test Setup

This is a temporary, isolated unified-schema test setup for validating the Inventory service before unified schema migration is complete. It deliberately does not modify the shared production schema artifacts or the default Kessel development stack.

Start it with:

```shell
make kessel-up-unified
```

The setup mounts `resources/` into the Inventory container and uses `configs/inventory-api.yaml`, which selects `in-memory.type: unified`.

The normal `make kessel-up` flow continues to use the legacy JSON schema cache. Note that the shared full-Kessel configuration enables the consumer's `load-reporter-representation` setting (required for reporter-side relationship tuples, and matching the stage configuration); deployments that have not opted in keep the setting disabled.

## Files

- `configs/inventory-api.yaml` selects unified YAML loading for the local stack.
- `resources/*.yaml` contains unified test artifacts for host, workspace, service, billing_account, k8s_cluster, k8s_policy, and notifications_integration.
- `docker-compose.override.yaml` mounts the temporary config and resource path.
- `kessel-inventory-ephem-unified.yaml` is a development-only ephemeral template that accepts a parameterized unified-schema tarball.
- `features-service-schema-tester.sh` is an automated end-to-end test for the Features service model, usable against both the JSON and unified schema stacks (see below).

These artifacts are local test fixtures. They are not part of the production schema set and should not be copied into `data/schema/resources`.

> [!NOTE]
> Some of the test resources (k8s-cluster, k8s-policy, notifications_integration) are there for validating schema loading but are not defined in the default schema.zed used in this compose setup. If you wish to test those resources, you'll need to create a custom schema.zed from the [current stage config](https://raw.githubusercontent.com/project-kessel/rbac-config/refs/heads/master/configs/stage/schemas/schema.zed) and add the missing definitions (available in [./deploy/schema.zed](../../deploy/schema.zed)), then update the **SCHEMA_ZED_FILE** variable in the full-kessel [.env](../full-kessel/.env) file to use it

## Features Service Schema Tester

`features-service-schema-tester.sh` runs an automated end-to-end test of the Features service enhancements being developed alongside unified schema including the relationship delta endpoints. The script seeds a workspace hierarchy in SpiceDB, reports billing accounts and workspaces, exercises `AddResourceRelationships` (including auto-creating a never-reported resource) and `RemoveResourceRelationships`, and verifies both the `enabled_services` permission checks and the underlying SpiceDB tuples with PASS/FAIL assertions. It exits non-zero when any assertion fails.

The same script works against both schema modes - start one or the other, then run the tester against it (the argument is the Inventory API gRPC port):

```shell
make kessel-up          # legacy JSON schema stack
# or
make kessel-up-unified  # unified schema stack

./development/unified-schema/features-service-schema-tester.sh 9081
```

Make sure to run the tester against a clean stack: some changes may persist between runs, and on a re-run the auto-creation check silently degrades into a merge-into-existing test. Use `make kessel-down` followed by `make kessel-up` (or `kessel-up-unified`) to reset.

The script requires `grpcurl` on PATH and `zed` configured (`zed context set`) against the stack's SpiceDB instance. Reporter-side relationship tuples require the consumer's `load-reporter-representation` setting, which both local stack configs already enable.

Note that strict attribute-field rejection on the delta endpoints is enforced only under unified schema; the JSON schema stack skips that check, so the script's assertions pass under both modes but only the unified stack rejects non-relationship fields.

For a similar step-by-step manual walkthrough of the same API surface - including a variant that sets up workspaces through RBAC instead of zed - see the [Features Service Local Testing guide](../../docs/dev-guides/features-service-local-testing.md).

## Ephemeral Tarball

The ephemeral template expects the unified YAML files in a base64-encoded tarball passed through the `UNIFIED_RESOURCES_TARBALL` template parameter.

Configure Bonfire to use this local template. Add or update the Kessel Inventory component in `~/.config/bonfire/config.yaml`:

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

The `--set-parameter` syntax is useful for the tarball because the base64 value should remain local and should not be committed to the Bonfire config. Other values can be defined in the config if preferred.

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

Once unified schema migration is complete, the approved unified schema artifacts and configuration should move into the common deployment and `kessel-up` setup. This temporary directory, its compose override, and its ephemeral template should then be deprecated and removed.

The full-Kessel stack uses the image configured by `development/full-kessel/.env`. To test a locally built image, edit that file to set `INVENTORY_API_IMAGE` - passing the variable on the `make` command line does not work, because the startup script loads the `.env` values over the caller's environment. Set `COMPOSE_PULL_MODE=missing` in the same file as well when the image only exists locally, so compose does not try to pull it.
