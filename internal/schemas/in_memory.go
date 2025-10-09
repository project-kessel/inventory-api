package schemas

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"os"
	"path/filepath"
	"sync"
)

type InMemorySchemaRepository struct {
	content sync.Map
}

func schemaTypeKey(schemaType SchemaType) (string, error) {
	if schemaType.ResourceType == "" {
		return "", fmt.Errorf("invalid schema type with empty ResourceType: %v", schemaType)
	}
	if schemaType.Type == "common" || schemaType.Type == "config" {
		return fmt.Sprintf("%s:%s", schemaType.Type, schemaType.ResourceType), nil
	}

	if schemaType.ReporterType != "" {
		return fmt.Sprintf("%s:%s", schemaType.Type, schemaType.ReporterType), nil
	}

	return "", fmt.Errorf("invalid schema type: %v", schemaType)
}

func (o *InMemorySchemaRepository) Create(ctx context.Context, schemaType SchemaType, content string) error {
	schemaKey, err := schemaTypeKey(schemaType)
	if err != nil {
		return err
	}

	o.content.Store(schemaKey, content)
	return nil
}

func (o *InMemorySchemaRepository) Get(ctx context.Context, schemaType SchemaType) (string, error) {
	schemaKey, err := schemaTypeKey(schemaType)
	if err != nil {
		return "", err
	}

	if schema, ok := o.content.Load(schemaKey); ok {
		return schema.(string), nil
	}

	return "", fmt.Errorf("schema not found for key %v", schemaType)
}

func (o *InMemorySchemaRepository) Update(ctx context.Context, schemaType SchemaType, content string) error {
	schemaKey, err := schemaTypeKey(schemaType)
	if err != nil {
		return err
	}

	o.content.Store(schemaKey, content)
	return nil
}

func (o *InMemorySchemaRepository) Delete(ctx context.Context, schemaType SchemaType) error {
	schemaKey, err := schemaTypeKey(schemaType)
	if err != nil {
		return err
	}

	o.content.Delete(schemaKey)
	return nil
}

func NewInMemorySchemaRepositoryFromDir(ctx context.Context, resourceDir string) (*InMemorySchemaRepository, error) {
	resourceDirs, err := os.ReadDir(resourceDir)
	if err != nil {
		return nil, fmt.Errorf("no directories inside schema directory")
	}

	repository := InMemorySchemaRepository{}

	for _, dir := range resourceDirs {
		if !dir.IsDir() {
			continue
		}
		resourceType := middleware.NormalizeResourceType(dir.Name())

		// Load and store common resource schema
		commonResourceSchema, err := middleware.LoadCommonResourceDataSchema(resourceType, resourceDir)
		if err != nil {
			err = repository.Create(ctx, SchemaType{
				ResourceType: resourceType,
				Type:         "common",
			}, commonResourceSchema)

			if err != nil {
				return nil, err
			}
		}

		_, err = middleware.LoadConfigFile(resourceDir, resourceType)
		if err != nil {
			log.Errorf("Failed to load config file for '%s': %v", resourceType, err)
			return nil, err
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
			reporterSchema, isReporterSchemaExists, err := middleware.LoadResourceSchema(resourceType, reporterType, resourceDir)
			if err == nil && isReporterSchemaExists {
				err = repository.Create(ctx, SchemaType{
					ResourceType: resourceType,
					ReporterType: reporterType,
				}, reporterSchema)
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

func NewInMemorySchemaRepositoryFromJsonFile(ctx context.Context, jsonFile string) (*InMemorySchemaRepository, error) {
	jsonData, err := os.ReadFile(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema cache file: %w", err)
	}

	return NewInMemorySchemaRepositoryFromJsonBytes(ctx, jsonData)
}

func NewInMemorySchemaRepositoryFromJsonBytes(ctx context.Context, jsonBytes []byte) (*InMemorySchemaRepository, error) {
	repository := InMemorySchemaRepository{}

	jsonContent := make(map[string]interface{})
	err := json.Unmarshal(jsonBytes, &jsonContent)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema cache JSON: %w", err)
	}

	for key, value := range jsonContent {
		repository.content.Store(key, value)
	}

	return &repository, nil
}

//
//func NewFileSystemSchemaService(schemaDir string) *FileSystemSchemaService {
//	return &FileSystemSchemaService{schemaDir: schemaDir}
//}
//
//func (s *FileSystemSchemaService) ShallowValidate(context context.Context, resourceType string, common CommonRepresentation, reporter ReporterRepresentation) error {
//
//	return nil
//}

// CalculateTuples(context.Context, string, CommonRepresentation, ReporterRepresentation) ([]model.TuplesToReplicate, error)
