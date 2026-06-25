# Unified YAML Schema Format

This directory contains serialized YAML schemas for Kessel Inventory API resources.

## Overview

Each `.yaml` file in `resources/` defines a resource type with:
- **JSON Schema** for field validation (common + reporter-specific)
- **Relations** for authorization relationship generation

These schemas are loaded at runtime for:
1. Validating incoming resource data
2. Calculating authorization relationships for SpiceDB

## Format

The unified schema format is defined in `unified-schema-format.json` (JSON Schema meta-schema).

### Basic Structure

```yaml
schema_version: "1.0"
name: resource_name
description: "Human-readable description"

common:
  # JSON Schema for common fields (shared across all reporters)
  schema:
    type: object
    properties:
      workspace_id:
        type: string
    required:
      - workspace_id
  
  # Relations for relationship generation
  relations:
    - name: workspace
      target: rbac/workspace
      field: workspace_id
      cardinality: one

reporters:
  - name: reporter_name
    description: "Reporter description"
    # Optional reporter-specific schema
    schema:
      type: object
      properties:
        reporter_field:
          type: string
```

## Key Concepts

### Fields (JSON Schema)

Fields are defined using standard **JSON Schema Draft 7** syntax embedded in YAML:
- Full JSON Schema support (oneOf, anyOf, allOf, patterns, enums, etc.)
- Nested objects and arrays
- Custom formats (uuid, date-time, etc.)

Only include fields that are **referenced by relations**. Business data that doesn't participate in authorization relations should not be in the schema.

### Relations

Relations define how resources connect for authorization. Each relation definition describes the schema for creating **relationships** (instances of the relation):

```yaml
relations:
  - name: workspace           # Relation name (becomes t_workspace in SpiceDB)
    target: rbac/workspace    # Target resource type
    field: workspace_id       # Field containing the relation ID
    cardinality: one          # "one" (single value) or "many" (array)
    description: "..."        # Optional description
```

**Nullable relations** are derived automatically:
- If `field` is NOT in `required[]` → nullable relation
- If `field` IS in `required[]` → non-nullable relation

### Cardinality

- **`one`**: Field contains a single ID (e.g., `workspace_id: "ws-123"`)
- **`many`**: Field contains an array of IDs (e.g., `owner_ids: ["user-1", "user-2"]`)

## Resources

### Inventory Resources (4)
- `host.yaml` - Physical/virtual hosts (HBI reporter)
- `k8s_cluster.yaml` - Kubernetes clusters (ACM/ACS/OCM reporters)
- `k8s_policy.yaml` - Kubernetes policies (ACM reporter)
- `notifications_integration.yaml` - Notification integrations (Notifications reporter)

### RBAC Infrastructure Resources (9)
- `workspace.yaml` - Workspace hierarchy
- `tenant.yaml` - Tenant isolation
- `role_binding.yaml` - Role-to-principal bindings
- `role.yaml` - Role definitions
- `group.yaml` - Principal groups
- `platform.yaml` - Platform-level scope
- `user.yaml` - User principals
- `principal.yaml` - Generic principals
- `tag.yaml` - Resource tags

## Validation

Validate schema files against the meta-schema:

```bash
# Using a JSON Schema validator (e.g., ajv-cli)
ajv validate -s unified-schema-format.json -d "resources/*.yaml"
```

## Examples

### Minimal Schema (No Relations)

```yaml
schema_version: "1.0"
name: tag
description: "Tag for categorization"

common:
  schema:
    type: object
    properties: {}
  relations: []

reporters:
  - name: rbac
    description: "RBAC service reporter"
```

### Schema with Relations

```yaml
schema_version: "1.0"
name: workspace
description: "Workspace for resource organization"

common:
  schema:
    type: object
    properties:
      parent_id:
        type: string
      tenant_id:
        type: string
    required:
      - tenant_id
  
  relations:
    - name: parent
      target: rbac/workspace
      field: parent_id
      cardinality: one
      # nullable: true (derived - parent_id not in required[])
    
    - name: tenant
      target: rbac/tenant
      field: tenant_id
      cardinality: one
      # nullable: false (derived - tenant_id in required[])

reporters:
  - name: rbac
    description: "RBAC service reporter"
```

### Schema with Many Cardinality

```yaml
schema_version: "1.0"
name: group
description: "Group of principals"

common:
  schema:
    type: object
    properties:
      member_ids:
        type: array
        items:
          type: string
      owner_id:
        type: string
    required:
      - owner_id
  
  relations:
    - name: member
      target: rbac/principal
      field: member_ids
      cardinality: many    # Array of member IDs
    
    - name: owner
      target: rbac/tenant
      field: owner_id
      cardinality: one

reporters:
  - name: rbac
    description: "RBAC service reporter"
```

## Design Decisions

### Why Embedded JSON Schema?

- ✅ Standard, well-supported format
- ✅ No custom type system to maintain
- ✅ Direct use with validation libraries (gojsonschema)
- ✅ Full expressiveness (oneOf, anyOf, patterns, etc.)

### Why Separate Relations Section?

- ✅ Authorization logic distinct from validation schema
- ✅ Explicit relationship generation rules
- ✅ Clear mapping to SpiceDB relations

### Why Derive Nullable from required[]?

- ✅ Single source of truth
- ✅ No redundancy or drift
- ✅ Simpler schema definitions

## See Also

- [YAML Schema Specification](../../docs/yaml-schema-spec.md) - Detailed spec
- [unified-schema-format.json](unified-schema-format.json) - Meta-schema (JSON Schema)
