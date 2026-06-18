package model

// UnifiedSchema represents a single resource schema loaded from a YAML file.
// Each YAML file contains one resource definition (e.g., host.yaml, k8s_cluster.yaml).
// This is the compiled/serialized format that embeds standard JSON Schema directly.
type UnifiedSchema struct {
	SchemaVersion string               `yaml:"schema_version"`
	Name          string               `yaml:"name"`
	Description   string               `yaml:"description"`
	Common        CommonDefinition     `yaml:"common"`
	Reporters     []ReporterDefinition `yaml:"reporters"`
}

// CommonDefinition holds the common (shared) schema and relations for a resource.
type CommonDefinition struct {
	// Schema is the embedded JSON Schema (as a map) for validation.
	// This is used directly with gojsonschema - no conversion needed.
	Schema map[string]interface{} `yaml:"schema"`

	// Relations define how this resource connects to other resources for authorization.
	Relations []RelationDefinition `yaml:"relations"`
}

// ReporterDefinition holds reporter-specific schema and relations.
type ReporterDefinition struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`

	// Schema is the embedded JSON Schema (as a map) for validation.
	// This is used directly with gojsonschema - no conversion needed.
	Schema map[string]interface{} `yaml:"schema"`

	// Relations define reporter-specific relations (Phase 4).
	// For Phase 1, this will be empty - only common relations are supported.
	Relations []RelationDefinition `yaml:"relations,omitempty"`
}

// RelationDefinition defines a connection between resources for tuple generation.
type RelationDefinition struct {
	// Name is the relation name used in SpiceDB tuples (e.g., "workspace", "tenant").
	Name string `yaml:"name"`

	// Target is the target resource type in "namespace/resource" format (e.g., "rbac/workspace").
	Target string `yaml:"target"`

	// Field is the field name in the representation containing the target ID (e.g., "workspace_id").
	Field string `yaml:"field"`

	// Cardinality is "one" for single values or "many" for arrays (Phase 4).
	// Phase 1 only supports "one".
	Cardinality string `yaml:"cardinality"`

	// Nullable indicates if the field can be null (Phase 4).
	// If true and field is null/empty, no tuple is created.
	Nullable bool `yaml:"nullable,omitempty"`
}

// ResourceType returns the ResourceType for this schema.
func (s UnifiedSchema) ResourceType() (ResourceType, error) {
	return NewResourceType(s.Name)
}

// ReporterType returns the ReporterType for this reporter definition.
func (r ReporterDefinition) ReporterType() (ReporterType, error) {
	return NewReporterType(r.Name)
}
