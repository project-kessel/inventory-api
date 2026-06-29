# Unified YAML Schema Specification

**Version**: 1.0  
**Status**: Phase 0 - Initial Definition  
**Date**: 2026-06-17

## Overview

This document defines the unified YAML schema format used by the Inventory API. This format is a **compiled/serialized artifact** that serves as the runtime input for schema validation and tuple generation.

## Purpose

The unified YAML schema replaces:
- JSON Schema files (`common_representation.json`, `reporters/{reporter}/{resource}.json`)
- Config files (`config.yaml`)
- Hardcoded tuple logic in `DefaultSchema.CalculateTuples()`

## Key Design: Embedded JSON Schema

The unified format **embeds standard JSON Schema directly** in YAML, with a custom `relations` section for tuple generation:

✅ **No custom type system** - Use JSON Schema standard directly  
✅ **No conversion needed** - JSON Schema is used as-is for validation  
✅ **Full JSON Schema support** - All features available (oneOf, anyOf, allOf, patterns, etc.)  
✅ **Simple implementation** - Extract schema and pass to `gojsonschema`

## File Structure

### Location
Compiled YAML schemas are stored at: `data/schema/resources/{resource_name}.yaml`

Example:
```
data/schema/resources/
  host.yaml
  k8s_cluster.yaml
  k8s_policy.yaml
  notifications_integration.yaml
```

### Top-Level Structure

```yaml
schema_version: "1.0"
name: {resource_name}
description: "{human-readable description}"

common:
  # Standard JSON Schema (in YAML format)
  schema:
    type: object
    properties: {...}
    required: [...]
  
  # Custom section for tuple generation
  relations: [...]

reporters:
  - name: {reporter_name}
    description: "{reporter description}"
    # Standard JSON Schema (in YAML format)
    schema:
      type: object
      properties: {...}
      required: [...]
    # Custom section for tuple generation (Phase 4)
    relations: [...]
```

**Note**: Each file defines a single resource (not wrapped in a `resources[]` array).

## Schema Definition (Embedded JSON Schema)

The `common.schema` and `reporters[].schema` sections use **standard JSON Schema** in YAML format.

### Basic Schema Structure

```yaml
common:
  schema:
    type: object
    properties:
      workspace_id:
        type: string
      display_name:
        type: string
        minLength: 1
        maxLength: 255
    required:
      - workspace_id
```

### JSON Schema Reference

All **standard JSON Schema Draft 7** features are supported:

- **Types**: `string`, `integer`, `number`, `boolean`, `array`, `object`, `null`
- **Formats**: `uuid`, `date-time`, `email`, `uri`, etc.
- **Validation**: `minLength`, `maxLength`, `minimum`, `maximum`, `pattern`, `enum`
- **Composition**: `oneOf`, `anyOf`, `allOf`, `not`
- **Required fields**: Listed in top-level `required` array

See [JSON Schema Documentation](https://json-schema.org/understanding-json-schema/) for full reference.

### Common Examples

#### String with Constraints
```yaml
properties:
  display_name:
    type: string
    minLength: 1
    maxLength: 255
    pattern: "^[a-zA-Z0-9-]+$"
```

#### UUID Field
```yaml
properties:
  id:
    type: string
    format: uuid
```

#### Enum
```yaml
properties:
  status:
    type: string
    enum:
      - READY
      - FAILED
      - OFFLINE
```

#### Nullable Field (oneOf)
```yaml
properties:
  satellite_id:
    oneOf:
      - type: string
        format: uuid
      - type: "null"
```

#### Array of Objects
```yaml
properties:
  nodes:
    type: array
    items:
      type: object
      properties:
        name:
          type: string
        cpu:
          type: string
      required:
        - name
        - cpu
```

#### Nested Object
```yaml
properties:
  system_profile:
    type: object
    properties:
      os_release:
        type: string
      arch:
        type: string
        enum: [x86_64, aarch64, ppc64le, s390x]
```

## Relations

Relations define how resources connect to each other. Each relation definition describes the schema for creating **relationships** (instances of the relation) for authorization.

### Basic Relation

```yaml
relations:
  - name: workspace
    target: rbac/workspace
    field: workspace_id
    cardinality: one
    description: "Workspace relation for authorization"
```

### Relation Properties

| Property | Required | Description | Example |
|----------|----------|-------------|---------|
| `name` | Yes | Relation name (used in SpiceDB) | `workspace`, `tenant` |
| `target` | Yes | Target resource type | `rbac/workspace`, `rbac/tenant` |
| `field` | Yes | Field name containing the ID | `workspace_id`, `tenant_id` |
| `cardinality` | Yes | `one` or `many` | `one` (single value), `many` (array) |
| `description` | No | Human-readable description | `"Workspace relation for authorization"` |

**Note**: Nullable is **derived** from the JSON Schema `required` array. If the `field` is NOT in `required[]`, the relation is nullable.

### Relation Location

**Phase 1**: Relations only in `common` section
**Phase 4**: Relations can be in `common` or per-reporter

```yaml
common:
  relations:
    - name: workspace      # All reporters use this
      target: rbac/workspace
      field: workspace_id

reporters:
  - name: ocm
    relations:             # Phase 4 - reporter-specific
      - name: subscription
        target: ocm/subscription
        field: subscription_id
```

### Field Extraction Rules

1. **Common relations** → Extract from common representation
2. **Reporter relations** (Phase 4) → Extract from reporter representation
3. **No fallback** in Phase 1 (might add in Phase 4)

## Complete Example

```yaml
# data/schema/resources/host.yaml
schema_version: "1.0"

resources:
  - name: host
    description: "A physical or virtual host in the inventory system"

    common:
      # Standard JSON Schema in YAML
      schema:
        type: object
        properties:
          workspace_id:
            type: string
        required:
          - workspace_id

      # Custom relations section
      relations:
        - name: workspace
          target: rbac/workspace
          field: workspace_id
          cardinality: one

    reporters:
      - name: hbi
        description: "Host-based inventory reporter"
        # Standard JSON Schema in YAML
        schema:
          type: object
          properties:
            satellite_id:
              oneOf:
                - type: string
                  format: uuid
                - type: "null"
            insights_id:
              oneOf:
                - type: string
                  format: uuid
                - type: "null"
            ansible_host:
              oneOf:
                - type: string
                  maxLength: 255
                - type: "null"
          required: []
```

## Multi-Reporter Example

```yaml
# data/schema/resources/k8s_cluster.yaml
schema_version: "1.0"

resources:
  - name: k8s_cluster
    description: "A Kubernetes cluster"

    common:
      # Standard JSON Schema
      schema:
        type: object
        properties:
          workspace_id:
            type: string
        required:
          - workspace_id

      # Custom relations section
      relations:
        - name: workspace
          target: rbac/workspace
          field: workspace_id
          cardinality: one

    reporters:
      - name: acm
        description: "Advanced Cluster Management reporter"
        # Standard JSON Schema
        schema:
          type: object
          properties:
            external_cluster_id:
              type: string
            cluster_status:
              type: string
              enum: [READY, FAILED, OFFLINE]
          required:
            - external_cluster_id
            - cluster_status

      - name: acs
        description: "Advanced Cluster Security reporter"
        # Same schema as ACM
        schema:
          type: object
          properties:
            external_cluster_id:
              type: string
            cluster_status:
              type: string
              enum: [READY, FAILED, OFFLINE]
          required:
            - external_cluster_id
            - cluster_status
```

## Using the Schema for Validation

The embedded JSON Schema is used **directly** with `gojsonschema` - no conversion needed.

### Implementation (Go)

```go
// Extract schema from YAML
commonSchema := resource.Common.Schema  // map[string]interface{}

// Validate data
schemaLoader := gojsonschema.NewGoLoader(commonSchema)
dataLoader := gojsonschema.NewGoLoader(representationData)
result, err := gojsonschema.Validate(schemaLoader, dataLoader)

if !result.Valid() {
    // Handle validation errors
}
```

### Equivalence to JSON Schema Files

The embedded YAML schema is **functionally identical** to the current JSON Schema files:

**Current (JSON file):**
```json
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "properties": {
    "workspace_id": {"type": "string"}
  },
  "required": ["workspace_id"]
}
```

**New (YAML embedded):**
```yaml
common:
  schema:
    type: object
    properties:
      workspace_id:
        type: string
    required:
      - workspace_id
```

Both produce the same validation behavior when loaded into `gojsonschema`.

## Comprehensive Example (All Features)

The following example demonstrates all implemented features including nullable relations, many cardinality, and reporter-specific relations:

```yaml
# data/schema/sample_app.yaml - Comprehensive example
schema_version: "1.0"
name: sample_app
description: "Sample application resource demonstrating all relation features"

common:
  schema:
    type: object
    properties:
      workspace_id:
        type: string
        description: "Required workspace - cardinality one"

      tenant_id:
        type: string
        description: "Optional tenant - nullable relation"

      tag_ids:
        type: array
        description: "Application tags - cardinality many"
        items:
          type: string

      owner_ids:
        type: array
        description: "Application owners - cardinality many"
        items:
          type: string

      app_name:
        type: string
        description: "Application name"

      environment:
        type: string
        enum:
          - development
          - staging
          - production
        description: "Deployment environment"

    required:
      - workspace_id
      - app_name
      - environment

  # Common relations - apply to all reporters
  relations:
    # Required one-to-one relation
    - name: workspace
      target: rbac/workspace
      field: workspace_id
      cardinality: one
      description: "Required workspace - non-nullable (workspace_id in required[])"

    # Optional one-to-one relation (nullable)
    - name: tenant
      target: rbac/tenant
      field: tenant_id
      cardinality: one
      description: "Optional tenant - nullable (tenant_id NOT in required[])"

    # Many-to-many relation for tags
    - name: tag
      target: rbac/tag
      field: tag_ids
      cardinality: many
      description: "Application tags - nullable (tag_ids NOT in required[])"

    # Many-to-many relation for owners
    - name: owner
      target: rbac/user
      field: owner_ids
      cardinality: many
      description: "Application owners - nullable (owner_ids NOT in required[])"

reporters:
  # OCM Reporter with reporter-specific relations
  - name: ocm
    description: "OpenShift Cluster Manager reporter"
    schema:
      type: object
      properties:
        subscription_id:
          type: string
          description: "OCM subscription ID"

        cluster_ids:
          type: array
          description: "List of cluster IDs where app is deployed"
          items:
            type: string

        deployment_count:
          type: integer
          description: "Number of deployments"

        last_sync:
          type: string
          format: date-time
          description: "Last sync timestamp"

      required:
        - subscription_id

    # Reporter-specific relations
    relations:
      # One-to-one relation from reporter data
      - name: subscription
        target: ocm/subscription
        field: subscription_id
        cardinality: one
        description: "OCM subscription - non-nullable (subscription_id in required[])"

      # Many-to-many relation from reporter data
      - name: cluster
        target: ocm/cluster
        field: cluster_ids
        cardinality: many
        description: "OCM clusters - nullable (cluster_ids NOT in required[])"

  # HBI Reporter with reporter-specific relations
  - name: hbi
    description: "Host-Based Inventory reporter"
    schema:
      type: object
      properties:
        host_ids:
          type: array
          description: "List of host IDs where app is installed"
          items:
            type: string

        installation_method:
          type: string
          enum:
            - rpm
            - container
            - source
          description: "How the app was installed"

        version:
          type: string
          description: "Application version"

      required:
        - installation_method

    # Reporter-specific relations
    relations:
      # Many-to-many relation from reporter data
      - name: host
        target: hbi/host
        field: host_ids
        cardinality: many
        description: "HBI hosts - nullable (host_ids NOT in required[])"
```

This example demonstrates:
- ✅ **Nullable relations**: `tenant_id` not in `required[]` → nullable
- ✅ **Many cardinality**: `tag_ids`, `owner_ids` as arrays → creates one relationship per element
- ✅ **Reporter-specific relations**: OCM has `subscription` and `cluster` relations, HBI has `host` relation
- ✅ **Mixed cardinality**: `workspace` (one), `tag` (many) in same resource
- ✅ **Derived nullability**: No explicit `nullable` field needed

Relations in the YAML schema drive relationship creation/deletion.

### Algorithm

**For cardinality "one":**
```
For each relation in schema.common.relations:
  1. Extract field value from current representation (e.g., workspace_id)
  2. Extract field value from previous representation
  3. If values changed:
     a. Create relationship: resource -[relation]-> target (current value)
     b. Delete relationship: resource -[relation]-> target (previous value)
  4. If relation is nullable and value is empty, skip
```

**For cardinality "many":**
```
For each relation with cardinality: many:
  1. Extract array values from current representation (e.g., tag_ids: ["tag-1", "tag-2"])
  2. Extract array values from previous representation (e.g., ["tag-2", "tag-3"])
  3. Calculate set difference:
     a. Create relationships for new elements: ["tag-1"]
     b. Delete relationships for removed elements: ["tag-3"]
     c. Keep unchanged elements: ["tag-2"]
```

### Example (cardinality: one)

Given relation:
```yaml
relations:
  - name: workspace
    target: rbac/workspace
    field: workspace_id
    cardinality: one
```

And resource change:
```
Previous: workspace_id = "ws-old"
Current:  workspace_id = "ws-new"
```

Generates relationships:
```
CREATE: hbi:resource-123#workspace@rbac:ws-new
DELETE: hbi:resource-123#workspace@rbac:ws-old
```

### Example (cardinality: many)

Given relation:
```yaml
relations:
  - name: tag
    target: rbac/tag
    field: tag_ids
    cardinality: many
```

And resource change:
```
Previous: tag_ids = ["tag-1", "tag-2"]
Current:  tag_ids = ["tag-2", "tag-3"]
```

Generates relationships:
```
CREATE: hbi:resource-123#tag@rbac:tag-3
DELETE: hbi:resource-123#tag@rbac:tag-1
(tag-2 unchanged, no action)
```

## Validation Rules

### Schema-Level Validation

1. `schema_version` must be present
2. `resources` must be an array with at least one resource
3. Each resource must have a `name`
4. Each resource must have at least one reporter

### Field-Level Validation

1. Field `name` is required
2. Field `type` must be a valid type
3. `required` defaults to `false` if not specified
4. `nullable` defaults to `false` if not specified
5. Enum values must match the field type
6. Array fields must specify `items`
7. Object fields must specify `properties`

### Relation-Level Validation

1. Relation `name` is required
2. Relation `target` is required and must be valid format (`namespace/resource`)
3. Relation `field` is required and must reference an existing field
4. Relation `cardinality` must be `one` or `many`
5. Field referenced by relation must exist in the same scope (common or reporter)

## Phase 1 Limitations

- **Top-level fields only** - No nested field references (`metadata.tenant_id`)
- **Common relations only** - Reporters cannot define relations yet
- **Cardinality: one only** - No array field support
- **Nullable not implemented** - All relations are required
- **Single relation** - Only workspace relation in practice

## Phase 4 Future Features

- **Multiple relations** - workspace, tenant, parent, bindings
- **Nullable relations** - Optional relation fields
- **Array fields** - cardinality: many support
- **Nested field references** - dot notation for nested access
- **Reporter-specific relations** - Relations defined per-reporter
- **Cross-resource validation** - Validate relation targets exist

## References

- Migration Plan: `SCHEMA_MIGRATION_PLAN.md`
- Current JSON Schemas: `data/schema/resources/{resource}/`
- Research: `/home/josejulio/Documents/redhat/research/RHCLOUD-48142/`
- YAML Format Examples: Research repository `examples/*.yaml`
