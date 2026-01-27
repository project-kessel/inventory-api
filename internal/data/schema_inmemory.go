package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type InMemorySchemaRepository struct {
	// TODO: Not thread safe - a sync.Map might not help either as we have to sync reporters (see UpdateResourceSchema) as well
	content map[model.ResourceType]*resourceEntry
}

type resourceEntry struct {
	model.ResourceSchemaRepresentation
	reporters map[model.ReporterType]*reporterEntry
}

type reporterEntry struct {
	model.ReporterSchemaRepresentation
}

func (o *InMemorySchemaRepository) GetResourceSchemas(ctx context.Context) ([]model.ResourceType, error) {
	var resourceTypes []model.ResourceType
	for resourceType := range o.content {
		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (o *InMemorySchemaRepository) CreateResourceSchema(ctx context.Context, resource model.ResourceSchemaRepresentation) error {
	if _, ok := o.content[resource.ResourceType]; ok {
		return fmt.Errorf("resource %s already exists", resource.ResourceType)
	}

	o.content[resource.ResourceType] = &resourceEntry{
		ResourceSchemaRepresentation: resource,
		reporters:                    map[model.ReporterType]*reporterEntry{},
	}
	return nil
}

func (o *InMemorySchemaRepository) GetResourceSchema(ctx context.Context, resourceType model.ResourceType) (model.ResourceSchemaRepresentation, error) {
	resource, ok := o.content[resourceType]
	if !ok {
		return model.ResourceSchemaRepresentation{}, model.ResourceSchemaNotFound
	}

	return resource.ResourceSchemaRepresentation, nil
}

func (o *InMemorySchemaRepository) UpdateResourceSchema(ctx context.Context, resource model.ResourceSchemaRepresentation) error {
	entry, ok := o.content[resource.ResourceType]
	if !ok {
		return model.ResourceSchemaNotFound
	}

	o.content[resource.ResourceType] = &resourceEntry{
		ResourceSchemaRepresentation: resource,
		reporters:                    entry.reporters,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteResourceSchema(ctx context.Context, resourceType model.ResourceType) error {
	if _, ok := o.content[resourceType]; !ok {
		return model.ResourceSchemaNotFound
	}

	delete(o.content, resourceType)
	return nil
}

func (o *InMemorySchemaRepository) GetReporterSchemas(ctx context.Context, resourceType model.ResourceType) ([]model.ReporterType, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return nil, err
	}

	var reporters []model.ReporterType
	for _, reporter := range entry.reporters {
		reporters = append(reporters, reporter.ReporterType)
	}

	return reporters, nil
}

func (o *InMemorySchemaRepository) CreateReporterSchema(ctx context.Context, resourceReporter model.ReporterSchemaRepresentation) error {
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

func (o *InMemorySchemaRepository) GetReporterSchema(ctx context.Context, resourceType model.ResourceType, reporterType model.ReporterType) (model.ReporterSchemaRepresentation, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return model.ReporterSchemaRepresentation{}, err
	}

	reporter, ok := entry.reporters[reporterType]
	if !ok {
		return model.ReporterSchemaRepresentation{}, model.ReporterSchemaNotFound
	}

	return reporter.ReporterSchemaRepresentation, nil
}

func (o *InMemorySchemaRepository) UpdateReporterSchema(ctx context.Context, resourceReporter model.ReporterSchemaRepresentation) error {
	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[resourceReporter.ReporterType]; !ok {
		return model.ReporterSchemaNotFound
	}

	entry.reporters[resourceReporter.ReporterType] = &reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteReporterSchema(ctx context.Context, resourceType model.ResourceType, reporterType model.ReporterType) error {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[reporterType]; !ok {
		return model.ReporterSchemaNotFound
	}

	delete(entry.reporters, reporterType)

	return nil
}

func (o *InMemorySchemaRepository) getResourceEntry(resourceType model.ResourceType) (*resourceEntry, error) {
	if entry, ok := o.content[resourceType]; ok {
		return entry, nil
	}

	return nil, model.ResourceSchemaNotFound
}

func NewInMemorySchemaRepository() *InMemorySchemaRepository {
	return &InMemorySchemaRepository{
		content: map[model.ResourceType]*resourceEntry{},
	}
}

func NewInMemorySchemaRepositoryFromDir(ctx context.Context, resourceDir string, validationSchemaFromString model.SchemaFromString) (*InMemorySchemaRepository, error) {
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema directory %q: %w", resourceDir, err)
	}

	repository := InMemorySchemaRepository{
		content: map[model.ResourceType]*resourceEntry{},
	}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType, err := model.NewResourceType(dir.Name())
		if err != nil {
			log.Warnf("Skipping invalid resource type directory '%s': %v", dir.Name(), err)
			continue
		}

		// Load and store common resource schema
		commonResourceSchema, err := loadCommonResourceDataSchema(resourceType.String(), resourceDir)
		if err == nil {
			err = repository.CreateResourceSchema(ctx, model.ResourceSchemaRepresentation{
				ResourceType:     resourceType,
				ValidationSchema: validationSchemaFromString(commonResourceSchema),
			})

			if err != nil {
				return nil, err
			}
		}

		reportersDir := filepath.Join(resourceDir, resourceType.String(), "reporters")
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
			reporterType, err := model.NewReporterType(reporter.Name())
			if err != nil {
				log.Warnf("Skipping invalid reporter type directory '%s': %v", reporter.Name(), err)
				continue
			}
			reporterSchema, isReporterSchemaExists, err := loadResourceSchema(resourceType.String(), reporterType.String(), resourceDir)
			if err == nil && isReporterSchemaExists {
				err = repository.CreateReporterSchema(ctx, model.ReporterSchemaRepresentation{
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

func NewInMemorySchemaRepositoryFromJsonFile(ctx context.Context, jsonFile string, validationSchemaFromString model.SchemaFromString) (*InMemorySchemaRepository, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema cache file: %w", err)
	}

	return NewFromJsonBytes(ctx, jsonData, validationSchemaFromString)
}

func NewFromJsonBytes(ctx context.Context, jsonBytes []byte, validationSchemaFromString model.SchemaFromString) (*InMemorySchemaRepository, error) {
	repository := InMemorySchemaRepository{
		content: map[model.ResourceType]*resourceEntry{},
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
			resourceTypeStr := key[len(commonPrefix):]
			resourceType, err := model.NewResourceType(resourceTypeStr)
			if err != nil {
				log.Warnf("Skipping invalid resource type in JSON '%s': %v", resourceTypeStr, err)
				continue
			}
			err = repository.CreateResourceSchema(ctx, model.ResourceSchemaRepresentation{
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
		resourceType, remainder := findResourceTypeFromJsonKey(key, resourceTypes)
		if resourceType == "" {
			continue
		}

		reporterType, err := model.NewReporterType(remainder)
		if err != nil {
			log.Warnf("Skipping invalid reporter type in JSON '%s': %v", remainder, err)
			continue
		}
		rt, _ := model.NewResourceType(string(resourceType))
		err = repository.CreateReporterSchema(ctx, model.ReporterSchemaRepresentation{
			ResourceType:     rt,
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

func findResourceTypeFromJsonKey(jsonKey string, resourceTypes []model.ResourceType) (model.ResourceType, string) {
	for _, resourceType := range resourceTypes {
		prefix := resourceType.String() + ":"
		if strings.HasPrefix(jsonKey, prefix) {
			return resourceType, jsonKey[len(prefix):]
		}
	}

	return "", ""
}
