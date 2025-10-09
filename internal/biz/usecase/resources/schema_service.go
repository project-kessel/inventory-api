package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
	"github.com/project-kessel/inventory-api/internal/data"
)

type SchemaUsecase struct {
	Log              *log.Helper
	schemaRepository schema.Repository
}

func NewSchemaUsecase(schemaRepository schema.Repository, logger *log.Helper) *SchemaUsecase {
	return &SchemaUsecase{
		Log:              logger,
		schemaRepository: schemaRepository,
	}
}

func (sc *SchemaUsecase) CalculateTuples(representations []data.RepresentationsByVersion, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	if len(representations) == 0 {
		return model.TuplesToReplicate{}, nil
	}

	// Identify current (max version) and previous (next lower version)
	var maxVer uint
	for _, r := range representations {
		if r.Version > maxVer {
			maxVer = r.Version
		}
	}
	var (
		currentWorkspaceID  string
		previousWorkspaceID string
	)
	for _, r := range representations {
		if r.Version == maxVer && currentWorkspaceID == "" {
			currentWorkspaceID = data.ExtractWorkspaceID(r)
		}
	}
	// find the largest version lower than maxVer
	var prevVer uint
	for _, r := range representations {
		if r.Version < maxVer && r.Version > prevVer {
			prevVer = r.Version
			previousWorkspaceID = data.ExtractWorkspaceID(r)
		}
	}

	return sc.BuildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID, key)
}

func (sc *SchemaUsecase) BuildTuplesToReplicate(currentWorkspaceID, previousWorkspaceID string, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	if previousWorkspaceID != "" && previousWorkspaceID == currentWorkspaceID {
		return model.TuplesToReplicate{}, nil
	}

	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, model.NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, model.NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// IsReporterForResource validates the resourceType and reporterType combination is valid. i.e. that there is a reporter that reports said resource.
func (sc *SchemaUsecase) IsReporterForResource(ctx context.Context, resourceType string, reporterType string) (bool, error) {
	if _, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType); err != nil {
		if errors.Is(err, schema.ResourceSchemaNotFound) || errors.Is(err, schema.ReporterSchemaNotfound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CommonShallowValidate validates the common representation for a given resourceType.
func (sc *SchemaUsecase) CommonShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}) error {
	resource, err := sc.schemaRepository.GetResourceSchema(ctx, resourceType)
	if err != nil {
		return fmt.Errorf("failed to load common representation schema for '%s': %w", resourceType, err)
	}

	if resource.ValidationSchema == nil {
		return fmt.Errorf("no schema found for '%s'", resourceType)
	}

	hasCommonRepresentationData := len(commonRepresentation) > 0
	if !hasCommonRepresentationData {
		commonRepresentation = map[string]interface{}{}
	}

	_, err = resource.ValidationSchema.Validate(commonRepresentation)
	if err != nil {
		if hasCommonRepresentationData {
			return err
		}
		return fmt.Errorf("missing 'common' field in payload - schema for '%s' has required fields: %w", resourceType, err)
	}

	return nil
}

// ReporterShallowValidate validates the specific reporter representation for a given resourceType/reporterType.
func (sc *SchemaUsecase) ReporterShallowValidate(ctx context.Context, resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error {
	reporter, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

	// Case 1: No schema found for resourceType:reporterType
	if reporter.ValidationSchema == nil {
		if len(reporterRepresentation) > 0 {
			return fmt.Errorf("no schema found for '%s:%s', but reporter representation was provided. Submission is not allowed", resourceType, reporterType)
		}
		log.Debugf("no schema found for %s:%s, treating as abstract reporter representation", resourceType, reporterType)
		return nil
	}

	hasReporterRepresentationData := len(reporterRepresentation) > 0
	if !hasReporterRepresentationData {
		reporterRepresentation = map[string]interface{}{}
	}

	_, err = reporter.ValidationSchema.Validate(reporterRepresentation)
	if err != nil {
		if hasReporterRepresentationData {
			return err
		}

		// If schema has validation errors but reporterRepresentation is nil/empty, that's an error
		return fmt.Errorf("missing 'reporter' field in payload - schema for '%s:%s' has required fields: %w", resourceType, reporterType, err)
	}

	return nil
}
