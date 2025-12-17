package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/biz/schema/validation"
)

type InMemorySchemaRepository struct {
	// TODO: Not thread safe - a sync.Map might not help either as we have to sync reporters (see UpdateResourceSchema) as well
	content map[string]*resourceEntry
}

type resourceEntry struct {
	schema.ResourceRepresentation
	reporters map[string]*reporterEntry
}

type reporterEntry struct {
	schema.ReporterRepresentation
}

func (o *InMemorySchemaRepository) GetResourceSchemas(ctx context.Context) ([]string, error) {
	var resourceTypes []string
	for resourceType := range o.content {
		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (o *InMemorySchemaRepository) CreateResourceSchema(ctx context.Context, resource schema.ResourceRepresentation) error {
	resource.ResourceType = NormalizeResourceType(resource.ResourceType)

	if _, ok := o.content[resource.ResourceType]; ok {
		return fmt.Errorf("resource %s already exists", resource.ResourceType)
	}

	o.content[resource.ResourceType] = &resourceEntry{
		ResourceRepresentation: resource,
		reporters:              map[string]*reporterEntry{},
	}
	return nil
}

func (o *InMemorySchemaRepository) GetResourceSchema(ctx context.Context, resourceType string) (schema.ResourceRepresentation, error) {
	resourceType = NormalizeResourceType(resourceType)

	resource, ok := o.content[resourceType]
	if !ok {
		return schema.ResourceRepresentation{}, schema.ResourceSchemaNotFound
	}

	return resource.ResourceRepresentation, nil
}

func (o *InMemorySchemaRepository) UpdateResourceSchema(ctx context.Context, resource schema.ResourceRepresentation) error {
	resource.ResourceType = NormalizeResourceType(resource.ResourceType)

	entry, ok := o.content[resource.ResourceType]
	if !ok {
		return schema.ResourceSchemaNotFound
	}

	o.content[resource.ResourceType] = &resourceEntry{
		ResourceRepresentation: resource,
		reporters:              entry.reporters,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteResourceSchema(ctx context.Context, resourceType string) error {
	resourceType = NormalizeResourceType(resourceType)

	if _, ok := o.content[resourceType]; !ok {
		return schema.ResourceSchemaNotFound
	}

	delete(o.content, resourceType)
	return nil
}

func (o *InMemorySchemaRepository) GetReporterSchemas(ctx context.Context, resourceType string) ([]string, error) {
	resourceType = NormalizeResourceType(resourceType)

	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return nil, err
	}

	var reporters []string
	for _, reporter := range entry.reporters {
		reporters = append(reporters, reporter.ReporterType)
	}

	return reporters, nil
}

func (o *InMemorySchemaRepository) CreateReporterSchema(ctx context.Context, resourceReporter schema.ReporterRepresentation) error {
	resourceReporter.ResourceType = NormalizeResourceType(resourceReporter.ResourceType)
	resourceReporter.ReporterType = normalizeReporterType(resourceReporter.ReporterType)

	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := (entry.reporters)[resourceReporter.ReporterType]; ok {
		return fmt.Errorf("reporter %s for entry %s already exist", resourceReporter.ReporterType, resourceReporter.ResourceType)
	}

	entry.reporters[resourceReporter.ReporterType] = &reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) GetReporterSchema(ctx context.Context, resourceType string, reporterType string) (schema.ReporterRepresentation, error) {
	resourceType = NormalizeResourceType(resourceType)
	reporterType = normalizeReporterType(reporterType)

	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return schema.ReporterRepresentation{}, err
	}

	reporter, ok := entry.reporters[reporterType]
	if !ok {
		return schema.ReporterRepresentation{}, schema.ReporterSchemaNotFound
	}

	return reporter.ReporterRepresentation, nil
}

func (o *InMemorySchemaRepository) UpdateReporterSchema(ctx context.Context, resourceReporter schema.ReporterRepresentation) error {
	resourceReporter.ResourceType = NormalizeResourceType(resourceReporter.ResourceType)
	resourceReporter.ReporterType = normalizeReporterType(resourceReporter.ReporterType)

	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[resourceReporter.ReporterType]; !ok {
		return schema.ReporterSchemaNotFound
	}

	entry.reporters[resourceReporter.ReporterType] = &reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteReporterSchema(ctx context.Context, resourceType string, reporterType string) error {
	resourceType = NormalizeResourceType(resourceType)
	reporterType = normalizeReporterType(reporterType)

	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[reporterType]; !ok {
		return schema.ReporterSchemaNotFound
	}

	delete(entry.reporters, reporterType)

	return nil
}

func (o *InMemorySchemaRepository) getResourceEntry(resourceType string) (*resourceEntry, error) {
	resourceType = NormalizeResourceType(resourceType)

	if entry, ok := o.content[resourceType]; ok {
		return entry, nil
	}

	return nil, schema.ResourceSchemaNotFound
}

func NewInMemorySchemaRepository() *InMemorySchemaRepository {
	return &InMemorySchemaRepository{
		content: map[string]*resourceEntry{},
	}
}

func NewInMemorySchemaRepositoryFromDir(ctx context.Context, resourceDir string, validationSchemaFromString validation.SchemaFromString) (*InMemorySchemaRepository, error) {
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory %q: %w", resourceDir, err)
	}

	repository := InMemorySchemaRepository{
		content: map[string]*resourceEntry{},
	}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType := NormalizeResourceType(dir.Name())

		// Load and store common resource schema
		commonResourceSchema, err := loadCommonResourceDataSchema(resourceType, resourceDir)
		if err == nil {
			err = repository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
				ResourceType:     resourceType,
				ValidationSchema: validationSchemaFromString(commonResourceSchema),
			})

			if err != nil {
				return nil, err
			}
		}

		reportersDir := filepath.Join(resourceDir, resourceType, "reporters")
		if _, err := os.Stat(reportersDir); os.IsNotExist(err) {
			continue
		}

		reporterDirs, err := os.ReadDir(reportersDir)
		if err != nil {
			log.Errorf("Failed to read reporters directory for '%s': %v", resourceType, err)
			continue
		}

		for _, reporter := range reporterDirs {
			if !reporter.IsDir() {
				continue
			}
			reporterType := reporter.Name()
			reporterSchema, isReporterSchemaExists, err := loadResourceSchema(resourceType, reporterType, resourceDir)
			if err == nil && isReporterSchemaExists {
				err = repository.CreateReporterSchema(ctx, schema.ReporterRepresentation{
					ResourceType:     resourceType,
					ReporterType:     reporterType,
					ValidationSchema: validationSchemaFromString(reporterSchema),
				})
				if err != nil {
					return nil, err
				}
			} else {
				log.Warnf("No schema found for %s:%s", resourceType, reporterType)
			}
		}
	}

	return &repository, nil
}

func NewInMemorySchemaRepositoryFromJsonFile(ctx context.Context, jsonFile string, validationSchemaFromString validation.SchemaFromString) (*InMemorySchemaRepository, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema cache file: %w", err)
	}

	return NewFromJsonBytes(ctx, jsonData, validationSchemaFromString)
}

func NewFromJsonBytes(ctx context.Context, jsonBytes []byte, validationSchemaFromString validation.SchemaFromString) (*InMemorySchemaRepository, error) {
	repository := InMemorySchemaRepository{
		content: map[string]*resourceEntry{},
	}

	jsonContent := make(map[string]interface{})
	err := json.Unmarshal(jsonBytes, &jsonContent)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema cache JSON: %w", err)
	}

	// Key format is as follows:
	// - common:{resource_type} -> common schema
	// - {resource_type}:{reporter_type} -> reporter schema
	// - config:{resource_type} -> config for resource
	// - config:{resource_type}:{reporter_type} -> config for resource's reporter
	// The config:* data does not appear to be used anywhere other than for the resource to know who
	//  are the valid reporters - This might be redundant now with `GetResourceReporters`

	commonPrefix := "common:"

	// Find the resources
	for key, value := range jsonContent {
		if strings.HasPrefix(key, commonPrefix) {
			resourceType := key[len(commonPrefix):]
			err := repository.CreateResourceSchema(ctx, schema.ResourceRepresentation{
				ResourceType:     resourceType,
				ValidationSchema: validationSchemaFromString(value.(string)),
			})

			if err != nil {
				return nil, err
			}
		}
	}

	resourceTypes, err := repository.GetResourceSchemas(ctx)
	if err != nil {
		return nil, err
	}

	// Find Reporters
	for key, value := range jsonContent {
		resourceType := findResourceTypeFromJsonKey(key, resourceTypes)
		if resourceType == "" {
			continue
		}

		reporterType := key[len(resourceType)+1:]
		err := repository.CreateReporterSchema(ctx, schema.ReporterRepresentation{
			ResourceType:     resourceType,
			ReporterType:     reporterType,
			ValidationSchema: validationSchemaFromString(value.(string)),
		})
		if err != nil {
			return nil, err
		}
	}

	return &repository, nil
}

func loadResourceSchema(resourceType string, reporterType string, dir string) (string, bool, error) {
	schemaPath := filepath.Join(dir, resourceType, "reporters", reporterType, fmt.Sprintf("%s.json", resourceType))

	// Check if file exists
	if _, err := os.Stat(schemaPath); err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("failed to check schema file for '%s': %w", resourceType, err)
	}

	// Read file
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", false, fmt.Errorf("failed to read schema file for '%s': %w", resourceType, err)
	}

	return string(data), true, nil
}

func loadCommonResourceDataSchema(resourceType string, baseSchemaDir string) (string, error) {

	schemaPath := filepath.Join(baseSchemaDir, resourceType, "common_representation.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read common resource schema: %w", err)
	}
	return string(data), nil
}

func NormalizeResourceType(resourceType string) string {
	return strings.ToLower(strings.ReplaceAll(resourceType, "/", "_"))
}

func normalizeReporterType(reporterType string) string {
	return strings.ToLower(reporterType)
}

func findResourceTypeFromJsonKey(jsonKey string, resourceTypes []string) string {
	for _, resourceType := range resourceTypes {
		if strings.HasPrefix(jsonKey, resourceType+":") {
			return resourceType
		}
	}

	return ""
}
