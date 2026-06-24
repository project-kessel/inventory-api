package data

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/xeipuuv/gojsonschema"
	"gopkg.in/yaml.v3"
)

// unifiedSchemaStructureSchema is the JSON Schema that validates the structure
// of our YAML schema files. This provides self-validation using the same
// JSON Schema approach we use for resource validation.
var unifiedSchemaStructureSchema = map[string]interface{}{
	"$schema":  "http://json-schema.org/draft-07/schema#",
	"type":     "object",
	"required": []interface{}{"schema_version", "name", "common", "reporters"},
	"properties": map[string]interface{}{
		"schema_version": map[string]interface{}{
			"type": "string",
		},
		"name": map[string]interface{}{
			"type":      "string",
			"minLength": 1,
		},
		"description": map[string]interface{}{
			"type": "string",
		},
		"common": map[string]interface{}{
			"type":     "object",
			"required": []interface{}{"schema"},
			"properties": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
				},
				"relations": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/definitions/relation",
					},
				},
			},
		},
		"reporters": map[string]interface{}{
			"type":     "array",
			"minItems": 1,
			"items": map[string]interface{}{
				"type":     "object",
				"required": []interface{}{"name", "schema"},
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":      "string",
						"minLength": 1,
					},
					"description": map[string]interface{}{
						"type": "string",
					},
					"schema": map[string]interface{}{
						"type": "object",
					},
					"relations": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/definitions/relation",
						},
					},
				},
			},
		},
	},
	"definitions": map[string]interface{}{
		"relation": map[string]interface{}{
			"type":     "object",
			"required": []interface{}{"name", "target", "field", "cardinality"},
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":      "string",
					"minLength": 1,
				},
				"target": map[string]interface{}{
					"type":      "string",
					"minLength": 1,
				},
				"field": map[string]interface{}{
					"type":      "string",
					"minLength": 1,
				},
				"cardinality": map[string]interface{}{
					"type": "string",
					"enum": []interface{}{"one", "many"},
				},
				"nullable": map[string]interface{}{
					"type": "boolean",
				},
			},
		},
	},
}

// LoadUnifiedSchemasFromDirectory loads all *.yaml schema files from the given directory.
// Each YAML file should contain a single UnifiedSchema (one resource per file).
//
// The directory structure should be:
//
//	data/schema/resources/
//	  host.yaml
//	  k8s_cluster.yaml
//	  k8s_policy.yaml
//
// Returns all loaded schemas or an error if any file fails to load.
func LoadUnifiedSchemasFromDirectory(dir string) ([]model.UnifiedSchema, error) {
	// Find all .yaml files in the directory
	pattern := filepath.Join(dir, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to list schema files in %q: %w", dir, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no .yaml schema files found in %q", dir)
	}

	var schemas []model.UnifiedSchema
	for _, file := range files {
		schema, err := LoadUnifiedSchemaFromFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to load schema from %q: %w", file, err)
		}
		schemas = append(schemas, schema)
	}

	return schemas, nil
}

// LoadUnifiedSchemaFromFile loads a single UnifiedSchema from a YAML file.
func LoadUnifiedSchemaFromFile(path string) (model.UnifiedSchema, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.UnifiedSchema{}, fmt.Errorf("failed to read file: %w", err)
	}

	var schema model.UnifiedSchema
	if err := yaml.Unmarshal(data, &schema); err != nil {
		return model.UnifiedSchema{}, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate the loaded schema structure using JSON Schema
	if err := validateUnifiedSchema(schema); err != nil {
		return model.UnifiedSchema{}, fmt.Errorf("schema validation failed: %w", err)
	}

	return schema, nil
}

// validateUnifiedSchema validates the schema structure using JSON Schema.
func validateUnifiedSchema(schema model.UnifiedSchema) error {
	// Convert schema to map for validation
	schemaMap := map[string]interface{}{
		"schema_version": schema.SchemaVersion,
		"name":           schema.Name,
		"description":    schema.Description,
		"common": map[string]interface{}{
			"schema":    schema.Common.Schema,
			"relations": convertRelationsToInterface(schema.Common.Relations),
		},
		"reporters": convertReportersToInterface(schema.Reporters),
	}

	// Validate using our structure schema
	schemaLoader := gojsonschema.NewGoLoader(unifiedSchemaStructureSchema)
	dataLoader := gojsonschema.NewGoLoader(schemaMap)

	result, err := gojsonschema.Validate(schemaLoader, dataLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		var errors string
		for _, err := range result.Errors() {
			errors += fmt.Sprintf("- %s\n", err)
		}
		return fmt.Errorf("schema structure validation failed:\n%s", errors)
	}

	return nil
}

// Helper functions to convert domain types to interface{} for JSON Schema validation

func convertRelationsToInterface(relations []model.RelationDefinition) []interface{} {
	result := make([]interface{}, len(relations))
	for i, r := range relations {
		result[i] = map[string]interface{}{
			"name":        r.Name,
			"target":      r.Target,
			"field":       r.Field,
			"cardinality": r.Cardinality,
			"nullable":    r.Nullable,
		}
	}
	return result
}

func convertReportersToInterface(reporters []model.ReporterDefinition) []interface{} {
	result := make([]interface{}, len(reporters))
	for i, r := range reporters {
		result[i] = map[string]interface{}{
			"name":        r.Name,
			"description": r.Description,
			"schema":      r.Schema,
			"relations":   convertRelationsToInterface(r.Relations),
		}
	}
	return result
}
