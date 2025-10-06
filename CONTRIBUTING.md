# Contributing

## Pull Requests
All pull requests should be made against the `main` branch from your own fork of the repository. Please ensure that your pull request includes a clear description of the changes made and all CI checks pass.

## Linting & Formatting

[golangci-lint](https://github.com/golangci/golangci-lint) should be used to lint this project. CI will automatically run this, but you can run linting locally by running:

```bash
make lint # requires docker/podman to be installed
```

### IDE Formatting

It is recommended to have `goimports` installed locally and have your IDE set to auto-format on save. You can install it by running:

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

For vscode users, you can set up your editor to use `goimports` for auto-formatting by adding the following to your workspace or user settings:

```json
{
  ...
  "go.formatTool": "goimports"
}
```

For jetbrains users see https://www.jetbrains.com/help/go/integration-with-go-tools.html#goimports

## Schema Changes

When making changes to schema configuration files in `data/schema/`, you **must** rebuild and commit the resources tarball. This ensures that schema changes are properly captured and consumable in deployments.

### Required Steps for Schema Changes

1. Make your changes to files in `data/schema/resources/`
2. Rebuild the tarball and update deployment configs:
   ```bash
   make build-schemas
   ```
3. Stage and commit both the schema changes and generated files:
   ```bash
   git add data/schema/ resources.tar.gz deploy/kessel-inventory-ephem.yaml
   git commit -m "Update schema: <description of changes>"
   ```

### Verification

Before pushing your PR, you can verify that your schema changes are properly synchronized:

```bash
./scripts/verify-schema-tarball.sh
```

This check is also enforced automatically in CI. PRs that modify schema files without updating the tarball will fail the `Verify Schema Tarball` check.

### Why This Matters

The `resources.tar.gz` file is used in production deployments as a ConfigMap. If schema changes aren't reflected in the tarball, those changes won't be available to running services, potentially causing deployment failures or runtime errors.
