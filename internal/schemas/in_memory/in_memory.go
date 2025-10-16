package in_memory

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/schemas/api"
	"os"
	"path/filepath"
	"strings"
)

type InMemorySchemaRepository struct {
	// TODO: Not thread safe - a sync.Map might not help either as we have to sync reporters (see UpdateResource) as well
	content map[string]resourceEntry
}

type resourceEntry struct {
	api.Resource
	reporters *map[string]reporterEntry
}

type reporterEntry struct {
	api.ResourceReporter
}

func (o *InMemorySchemaRepository) GetResources(ctx context.Context) ([]string, error) {
	var resourceTypes []string
	for resourceType := range o.content {
		resourceTypes = append(resourceTypes, resourceType)
	}

	return resourceTypes, nil
}

func (o *InMemorySchemaRepository) CreateResource(ctx context.Context, resource api.Resource) error {
	if _, ok := o.content[resource.ResourceType]; ok {
		return fmt.Errorf("resource %s already exists", resource.ResourceType)
	}

	o.content[resource.ResourceType] = resourceEntry{
		Resource:  resource,
		reporters: &map[string]reporterEntry{},
	}
	return nil
}

func (o *InMemorySchemaRepository) GetResource(ctx context.Context, resourceType string) (api.Resource, error) {
	resource, ok := o.content[resourceType]
	if !ok {
		return api.Resource{}, fmt.Errorf("resource %s does not exist", resourceType)
	}

	return resource.Resource, nil
}

func (o *InMemorySchemaRepository) UpdateResource(ctx context.Context, resource api.Resource) error {
	entry, ok := o.content[resource.ResourceType]
	if !ok {
		return fmt.Errorf("resource %s does not exist", resource.ResourceType)
	}

	o.content[resource.ResourceType] = resourceEntry{
		Resource:  resource,
		reporters: entry.reporters,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteResource(ctx context.Context, resourceType string) error {
	delete(o.content, resourceType)

	return nil
}

func (o *InMemorySchemaRepository) GetResourceReporters(ctx context.Context, resourceType string) ([]string, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return nil, err
	}

	var reporters []string
	for _, reporter := range *entry.reporters {
		reporters = append(reporters, reporter.ReporterType)
	}

	return reporters, nil
}

func (o *InMemorySchemaRepository) CreateResourceReporter(ctx context.Context, resourceReporter api.ResourceReporter) error {
	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := (*entry.reporters)[resourceReporter.ReporterType]; ok {
		return fmt.Errorf("reporter %s for entry %s already exist", resourceReporter.ReporterType, resourceReporter.ResourceType)
	}

	(*entry.reporters)[resourceReporter.ReporterType] = reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) GetResourceReporter(ctx context.Context, resourceType string, reporterType string) (api.ResourceReporter, error) {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return api.ResourceReporter{}, err
	}

	reporter, ok := (*entry.reporters)[reporterType]
	if !ok {
		return api.ResourceReporter{}, fmt.Errorf("invalid reporter_type: %s for resource_type: %s", reporterType, resourceType)
	}

	return reporter.ResourceReporter, nil
}

func (o *InMemorySchemaRepository) UpdateResourceReporter(ctx context.Context, resourceReporter api.ResourceReporter) error {
	entry, err := o.getResourceEntry(resourceReporter.ResourceType)
	if err != nil {
		return err
	}

	if _, ok := (*entry.reporters)[resourceReporter.ReporterType]; !ok {
		return fmt.Errorf("reporter %s does not exist", resourceReporter.ReporterType)
	}

	(*entry.reporters)[resourceReporter.ReporterType] = reporterEntry{
		resourceReporter,
	}

	return nil
}

func (o *InMemorySchemaRepository) DeleteResourceReporter(ctx context.Context, resourceType string, reporterType string) error {
	entry, err := o.getResourceEntry(resourceType)
	if err != nil {
		return err
	}

	if _, ok := (*entry.reporters)[reporterType]; !ok {
		return fmt.Errorf("reporter %s does not exist", reporterType)
	}

	delete(*entry.reporters, reporterType)

	return nil
}

func (o *InMemorySchemaRepository) getResourceEntry(resourceType string) (*resourceEntry, error) {
	if entry, ok := o.content[resourceType]; !ok {
		return nil, fmt.Errorf("resource type %s does not exist", resourceType)
	} else {
		return &entry, nil
	}
}

func New(ctx context.Context) *InMemorySchemaRepository {
	return &InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}
}

func NewFromDir(ctx context.Context, resourceDir string) (*InMemorySchemaRepository, error) {
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return nil, fmt.Errorf("no directories inside schema directory")
	}

	repository := InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
	}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType := normalizeResourceType(dir.Name())

		// Load and store common resource schema
		commonResourceSchema, err := loadCommonResourceDataSchema(resourceType, resourceDir)
		if err == nil {
			err = repository.CreateResource(ctx, api.Resource{
				ResourceType: resourceType,
				CommonSchema: commonResourceSchema,
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
				err = repository.CreateResourceReporter(ctx, api.ResourceReporter{
					ResourceType:   resourceType,
					ReporterType:   reporterType,
					ReporterSchema: reporterSchema,
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

func NewFromJsonFile(ctx context.Context, jsonFile string) (*InMemorySchemaRepository, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema cache file: %w", err)
	}

	return NewFromJsonBytes(ctx, jsonData)
}

func NewFromJsonBytes(ctx context.Context, jsonBytes []byte) (*InMemorySchemaRepository, error) {
	repository := InMemorySchemaRepository{
		content: make(map[string]resourceEntry),
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
			err := repository.CreateResource(ctx, api.Resource{
				ResourceType: resourceType,
				CommonSchema: value.(string),
			})

			if err != nil {
				return nil, err
			}
		}
	}

	resourceTypes, err := repository.GetResources(ctx)
	if err != nil {
		return nil, err
	}

	// Find Reporters
	for key, value := range jsonContent {
		for _, resourceType := range resourceTypes {
			if strings.HasPrefix(key, resourceType+":") {
				reporterType := key[len(resourceType)+1:]
				err := repository.CreateResourceReporter(ctx, api.ResourceReporter{
					ResourceType:   resourceType,
					ReporterType:   reporterType,
					ReporterSchema: value.(string),
				})
				if err != nil {
					return nil, err
				}

				break
			}
		}
	}

	return &repository, nil
}
