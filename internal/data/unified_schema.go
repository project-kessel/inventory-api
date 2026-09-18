package data

// UnifiedSchema is the serialized schema artifact loaded from YAML.
type UnifiedSchema struct {
	SchemaVersion string                            `yaml:"schema_version"`
	Name          string                            `yaml:"name"`
	Description   string                            `yaml:"description"`
	Common        UnifiedSchemaCommonDefinition     `yaml:"common"`
	Reporters     []UnifiedSchemaReporterDefinition `yaml:"reporters"`
}

// UnifiedSchemaCommonDefinition contains the common section of the unified schema.
// It contains the common representation and relations definitions used for authorization.
type UnifiedSchemaCommonDefinition struct {
	Schema    map[string]interface{}  `yaml:"schema"`
	Relations []UnifiedSchemaRelation `yaml:"relations"`
}

// UnifiedSchemaReporterDefinition contains reporters section of the unified schema.
// It contains the reporter definition and representation, and relations definitions
// used for authorization
type UnifiedSchemaReporterDefinition struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Schema      map[string]interface{}  `yaml:"schema"`
	Relations   []UnifiedSchemaRelation `yaml:"relations"`
}

// UnifiedSchemaRelation describes the relations used for authorization.
type UnifiedSchemaRelation struct {
	Name        string `yaml:"name"`
	Target      string `yaml:"target"`
	Field       string `yaml:"field"`
	Cardinality string `yaml:"cardinality"`
	Description string `yaml:"description,omitempty"`
}
