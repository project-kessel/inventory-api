package data

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

type InMemorySchemaRepository struct {
	// TODO: Not thread safe - a sync.Map might not help either as we have to sync reporters (see UpdateResourceSchema) as well
	content map[string]*resourceEntry
}

type resourceEntry struct {
	bizmodel.ResourceSchema
	reporters map[string]*reporterEntry
}

type reporterEntry struct {
	bizmodel.ReporterSchema
}

func (o *InMemorySchemaRepository) GetResourceSchemas(ctx context.Context) ([]bizmodel.ResourceType, error) {
	resourceTypes := make([]bizmodel.ResourceType, 0, len(o.content))
	for _, entry := range o.content {
		resourceTypes = append(resourceTypes, entry.ResourceType)
	}

	return resourceTypes, nil
}

func (o *InMemorySchemaRepository) CreateResourceSchema(ctx context.Context, resource bizmodel.ResourceSchema) error {
	if _, ok := o.content[resource.ResourceType.String()]; ok {
		return fmt.Errorf("resource %s already exists", resource.ResourceType)
	}

	o.content[resource.ResourceType.String()] = &resourceEntry{
		ResourceSchema: resource,
		reporters:      map[string]*reporterEntry{},
	}
	return nil
}

func (o *InMemorySchemaRepository) GetResourceSchema(ctx context.Context, resourceType bizmodel.ResourceType) (bizmodel.ResourceSchema, error) {
	resource, ok := o.content[resourceType.String()]
	if !ok {
		return bizmodel.ResourceSchema{}, bizmodel.ResourceSchemaNotFound
	}

	return resource.ResourceSchema, nil
}

func (o *InMemorySchemaRepository) UpdateResourceSchema(ctx context.Context, resource bizmodel.ResourceSchema) error {
	entry, ok := o.content[resource.ResourceType.String()]
	if !ok {
		return bizmodel.ResourceSchemaNotFound
	}

	o.content[resource.ResourceType.String()] = &resourceEntry{
		ResourceSchema: resource,
		reporters:      entry.reporters,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteResourceSchema(ctx context.Context, resourceType bizmodel.ResourceType) error {
	if _, ok := o.content[resourceType.String()]; !ok {
		return bizmodel.ResourceSchemaNotFound
	}

	delete(o.content, resourceType.String())
	return nil
}

func (o *InMemorySchemaRepository) GetReporterSchemas(ctx context.Context, resourceType bizmodel.ResourceType) ([]bizmodel.ReporterType, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return nil, err
	}

	reporters := make([]bizmodel.ReporterType, 0, len(entry.reporters))
	for _, reporter := range entry.reporters {
		reporters = append(reporters, reporter.ReporterType)
	}

	return reporters, nil
}

func (o *InMemorySchemaRepository) CreateReporterSchema(ctx context.Context, resourceReporter bizmodel.ReporterSchema) error {
	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[resourceReporter.ReporterType.String()]; ok {
		return fmt.Errorf("reporter %s for entry %s already exist", resourceReporter.ReporterType, resourceReporter.ResourceType)
	}

	entry.reporters[resourceReporter.ReporterType.String()] = &reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) GetReporterSchema(ctx context.Context, resourceType bizmodel.ResourceType, reporterType bizmodel.ReporterType) (bizmodel.ReporterSchema, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return bizmodel.ReporterSchema{}, err
	}

	reporter, ok := entry.reporters[reporterType.String()]
	if !ok {
		return bizmodel.ReporterSchema{}, bizmodel.ReporterSchemaNotFound
	}

	return reporter.ReporterSchema, nil
}

func (o *InMemorySchemaRepository) UpdateReporterSchema(ctx context.Context, resourceReporter bizmodel.ReporterSchema) error {
	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[resourceReporter.ReporterType.String()]; !ok {
		return bizmodel.ReporterSchemaNotFound
	}

	entry.reporters[resourceReporter.ReporterType.String()] = &reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteReporterSchema(ctx context.Context, resourceType bizmodel.ResourceType, reporterType bizmodel.ReporterType) error {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return err
	}

	if _, ok := entry.reporters[reporterType.String()]; !ok {
		return bizmodel.ReporterSchemaNotFound
	}

	delete(entry.reporters, reporterType.String())

	return nil
}

func (o *InMemorySchemaRepository) getResourceEntry(resourceType bizmodel.ResourceType) (*resourceEntry, error) {
	if entry, ok := o.content[resourceType.String()]; ok {
		return entry, nil
	}

	return nil, bizmodel.ResourceSchemaNotFound
}

func NewInMemorySchemaRepository() *InMemorySchemaRepository {
	return &InMemorySchemaRepository{
		content: map[string]*resourceEntry{},
	}
}

func NewInMemorySchemaRepositoryFromDir(ctx context.Context, resourceDir string, validationSchemaFromString bizmodel.ValidationSchemaFromString) (*InMemorySchemaRepository, error) {
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
		resourceType, err := bizmodel.NewResourceType(dir.Name())
		if err != nil {
			return nil, fmt.Errorf("invalid resource type directory %q: %w", dir.Name(), err)
		}
		// Load and store common resource schema
		commonResourceSchema, err := loadCommonResourceDataSchema(resourceType, resourceDir)
		if err == nil {
			err = repository.CreateResourceSchema(ctx, bizmodel.ResourceSchema{
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
			reporterType, err := bizmodel.NewReporterType(reporter.Name())
			if err != nil {
				return nil, fmt.Errorf("invalid reporter type %q: %w", reporter.Name(), err)
			}
			reporterSchema, isReporterSchemaExists, err := loadResourceSchema(resourceType, reporterType, resourceDir)
			if err == nil && isReporterSchemaExists {
				err = repository.CreateReporterSchema(ctx, bizmodel.ReporterSchema{
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

func NewInMemorySchemaRepositoryFromJsonFile(ctx context.Context, jsonFile string, validationSchemaFromString bizmodel.ValidationSchemaFromString) (*InMemorySchemaRepository, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema cache file: %w", err)
	}

	return NewFromJsonBytes(ctx, jsonData, validationSchemaFromString)
}

func NewFromJsonBytes(ctx context.Context, jsonBytes []byte, validationSchemaFromString bizmodel.ValidationSchemaFromString) (*InMemorySchemaRepository, error) {
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
			resourceTypeStr := key[len(commonPrefix):]
			rt, err := bizmodel.NewResourceType(resourceTypeStr)
			if err != nil {
				return nil, fmt.Errorf("invalid resource type in schema JSON key %q: %w", key, err)
			}
			err = repository.CreateResourceSchema(ctx, bizmodel.ResourceSchema{
				ResourceType:     rt,
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
		rt, ok := findResourceTypeFromJsonKey(key, resourceTypes)
		if !ok {
			continue
		}

		reporterType, err := bizmodel.NewReporterType(key[len(rt.String())+1:])
		if err != nil {
			return nil, fmt.Errorf("invalid reporter type in schema JSON key %q: %w", key, err)
		}
		err = repository.CreateReporterSchema(ctx, bizmodel.ReporterSchema{
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

func loadResourceSchema(resourceType bizmodel.ResourceType, reporterType bizmodel.ReporterType, dir string) (string, bool, error) {
	schemaPath := filepath.Join(dir, resourceType.String(), "reporters", reporterType.String(), fmt.Sprintf("%s.json", resourceType))

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

func loadCommonResourceDataSchema(resourceType bizmodel.ResourceType, baseSchemaDir string) (string, error) {
	schemaPath := filepath.Join(baseSchemaDir, resourceType.String(), "common_representation.json")

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read common resource schema: %w", err)
	}
	return string(data), nil
}


func findResourceTypeFromJsonKey(jsonKey string, resourceTypes []bizmodel.ResourceType) (bizmodel.ResourceType, bool) {
	for _, rt := range resourceTypes {
		if strings.HasPrefix(jsonKey, rt.String()+":") {
			return rt, true
		}
	}

	return bizmodel.ResourceType(""), false
}
