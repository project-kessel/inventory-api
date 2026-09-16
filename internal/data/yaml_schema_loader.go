package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// This embeds unified_schema_format.json into the compiled Go binary
// so the meta-schema can validate unified YAML artifact structure at runtime
// without reading a separate file from disk.
//
//go:embed unified_schema_format.json
var unifiedSchemaFormat []byte

var unifiedSchemaTargetPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*/[a-z][a-z0-9_]*$`)

// LoadUnifiedSchemasFromDirectory loads every YAML schema artifact directly in
// dir. A directory is loaded atomically: one invalid artifact fails the whole
// load rather than returning a partially usable schema set.
func LoadUnifiedSchemasFromDirectory(dir string) ([]UnifiedSchema, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return nil, fmt.Errorf("failed to list unified schemas in %q: %w", dir, err)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no unified YAML schemas found in %q", dir)
	}

	schemas := make([]UnifiedSchema, 0, len(paths))
	seenNames := make(map[string]string, len(paths))
	for _, path := range paths {
		schema, err := LoadUnifiedSchemaFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load unified schema %q: %w", path, err)
		}
		if previousPath, ok := seenNames[schema.Name]; ok {
			return nil, fmt.Errorf("duplicate unified schema name %q in %q and %q", schema.Name, previousPath, path)
		}
		seenNames[schema.Name] = path
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// LoadUnifiedSchemaFromFile parses and validates one schema artifact.
func LoadUnifiedSchemaFromFile(path string) (UnifiedSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return UnifiedSchema{}, fmt.Errorf("failed to read file: %w", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return UnifiedSchema{}, fmt.Errorf("failed to parse YAML: %w", err)
	}
	if err := validateUnifiedSchemaFormat(raw); err != nil {
		return UnifiedSchema{}, err
	}

	var schema UnifiedSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return UnifiedSchema{}, fmt.Errorf("failed to decode schema: %w", err)
	}

	filename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if schema.Name != filename {
		return UnifiedSchema{}, fmt.Errorf("schema name %q does not match filename %q", schema.Name, filename)
	}
	if err := validateEmbeddedSchemas(schema); err != nil {
		return UnifiedSchema{}, err
	}
	if err := validateRelationFields(schema); err != nil {
		return UnifiedSchema{}, err
	}

	return schema, nil
}

// validateUnifiedSchemaFormat validates raw YAML against the unified format meta-schema.
func validateUnifiedSchemaFormat(raw map[string]interface{}) error {
	if raw == nil {
		return fmt.Errorf("schema must be a YAML object")
	}

	formatLoader := gojsonschema.NewBytesLoader(unifiedSchemaFormat)
	data, err := json.Marshal(raw)
	if err != nil {
		return fmt.Errorf("failed to normalize schema for validation: %w", err)
	}
	result, err := gojsonschema.Validate(formatLoader, gojsonschema.NewBytesLoader(data))
	if err != nil {
		return fmt.Errorf("failed to validate schema format: %w", err)
	}
	if result.Valid() {
		return nil
	}

	errors := make([]string, 0, len(result.Errors()))
	for _, validationError := range result.Errors() {
		errors = append(errors, validationError.String())
	}
	return fmt.Errorf("schema format validation failed: %s", strings.Join(errors, "; "))
}

// validateEmbeddedSchemas compiles the common and reporter JSON Schemas.
func validateEmbeddedSchemas(schema UnifiedSchema) error {
	if err := compileEmbeddedSchema(schema.Name, "common", schema.Common.Schema); err != nil {
		return err
	}
	for _, reporter := range schema.Reporters {
		if reporter.Schema == nil {
			continue
		}
		if err := compileEmbeddedSchema(schema.Name, reporter.Name, reporter.Schema); err != nil {
			return err
		}
	}
	return nil
}

// compileEmbeddedSchema verifies that one embedded JSON Schema is valid.
func compileEmbeddedSchema(resourceName, scope string, schema map[string]interface{}) error {
	if err := rejectExternalSchemaReferences(schema, resourceName+":"+scope); err != nil {
		return err
	}
	if _, err := gojsonschema.NewSchema(gojsonschema.NewGoLoader(schema)); err != nil {
		return fmt.Errorf("invalid embedded JSON Schema for %s:%s: %w", resourceName, scope, err)
	}
	return nil
}

// rejectExternalSchemaReferences allows only local fragment references in a JSON Schema.
func rejectExternalSchemaReferences(value interface{}, path string) error {
	switch current := value.(type) {
	case map[string]interface{}:
		if rawReference, ok := current["$ref"]; ok {
			reference, ok := rawReference.(string)
			if !ok || !strings.HasPrefix(reference, "#") {
				return fmt.Errorf("external JSON Schema reference %q is not allowed at %s; only local fragment references are supported", rawReference, path)
			}
		}
		for key, child := range current {
			if err := rejectExternalSchemaReferences(child, path+"."+key); err != nil {
				return err
			}
		}
	case []interface{}:
		for index, child := range current {
			if err := rejectExternalSchemaReferences(child, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateRelationFields verifies relation references and cardinality field types.
func validateRelationFields(schema UnifiedSchema) error {
	if err := validateRelations(schema.Name, "common", schema.Common.Schema, schema.Common.Relations); err != nil {
		return err
	}
	for _, reporter := range schema.Reporters {
		if err := validateRelations(schema.Name, reporter.Name, reporter.Schema, reporter.Relations); err != nil {
			return err
		}
	}
	return nil
}

// validateRelations validates relations within one common or reporter section.
func validateRelations(resourceName, scope string, jsonSchema map[string]interface{}, relations []UnifiedSchemaRelation) error {
	if len(relations) == 0 {
		return nil
	}
	properties, ok := jsonSchema["properties"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("embedded JSON Schema for %s:%s must define properties", resourceName, scope)
	}

	seenRelations := make(map[string]struct{}, len(relations))
	for _, relation := range relations {
		if _, exists := seenRelations[relation.Name]; exists {
			return fmt.Errorf("duplicate relation %q in %s:%s", relation.Name, resourceName, scope)
		}
		seenRelations[relation.Name] = struct{}{}

		if !unifiedSchemaTargetPattern.MatchString(relation.Target) {
			return fmt.Errorf("invalid relation target %q in %s:%s", relation.Target, resourceName, scope)
		}
		fieldSchema, exists := properties[relation.Field]
		if !exists {
			return fmt.Errorf("relation %q in %s:%s references undefined field %q", relation.Name, resourceName, scope, relation.Field)
		}
		if err := validateRelationFieldType(relation, fieldSchema); err != nil {
			return fmt.Errorf("invalid relation %q in %s:%s: %w", relation.Name, resourceName, scope, err)
		}
	}
	return nil
}

// validateRelationFieldType verifies the JSON Schema type required by cardinality.
func validateRelationFieldType(relation UnifiedSchemaRelation, fieldSchema interface{}) error {
	schema, ok := fieldSchema.(map[string]interface{})
	if !ok {
		return fmt.Errorf("field %q must have a JSON Schema object", relation.Field)
	}

	fieldType, _ := schema["type"].(string)
	switch relation.Cardinality {
	case "one":
		if fieldType != "string" {
			return fmt.Errorf("field %q must have type string for cardinality one", relation.Field)
		}
	case "many":
		if fieldType != "array" {
			return fmt.Errorf("field %q must have type array for cardinality many", relation.Field)
		}
		items, ok := schema["items"].(map[string]interface{})
		if !ok || items["type"] != "string" {
			return fmt.Errorf("field %q must define string array items for cardinality many", relation.Field)
		}
	default:
		return fmt.Errorf("unsupported cardinality %q", relation.Cardinality)
	}
	return nil
}
