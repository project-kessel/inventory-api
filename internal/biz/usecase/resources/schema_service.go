package resources

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/schema"
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

func (sc *SchemaUsecase) CalculateTuples(currentRepresentation, previousRepresentation *model.Representations, key model.ReporterResourceKey) (model.TuplesToReplicate, error) {
	// Extract workspace IDs from representations
	// currentRepresentation can be nil for DELETE operations (meaning no current/new state)
	currentWorkspaceID := ""
	if currentRepresentation != nil {
		currentWorkspaceID = currentRepresentation.WorkspaceID()
	}
	previousWorkspaceID := ""
	if previousRepresentation != nil {
		previousWorkspaceID = previousRepresentation.WorkspaceID()
	}

	// Handle no-op case where workspace hasn't changed
	if previousWorkspaceID != "" && previousWorkspaceID == currentWorkspaceID {
		return model.TuplesToReplicate{}, nil
	}

	// Build tuples to create and delete
	var tuplesToCreate, tuplesToDelete []model.RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, model.NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, model.NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	return model.NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// ShallowValidate validates a ReportResourceCommand against schemas.
// It checks that the reporter is allowed for the resource type,
// and validates both reporter and common representations.
func (sc *SchemaUsecase) ShallowValidate(ctx context.Context, cmd ReportResourceCommand) error {
	resourceType := cmd.ResourceType.String()
	reporterType := cmd.ReporterType.String()

	if isReporter, err := sc.isReporterForResource(ctx, resourceType, reporterType); !isReporter {
		if err != nil {
			return err
		}
		return fmt.Errorf("reporter %s does not report resource types: %s", reporterType, resourceType)
	}

	if err := sc.reporterShallowValidate(ctx, resourceType, reporterType, cmd.ReporterRepresentation); err != nil {
		return err
	}

	if err := sc.commonShallowValidate(ctx, resourceType, cmd.CommonRepresentation); err != nil {
		return err
	}

	return nil
}

// isReporterForResource validates the resourceType and reporterType combination is valid.
func (sc *SchemaUsecase) isReporterForResource(ctx context.Context, resourceType string, reporterType string) (bool, error) {
	if _, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType); err != nil {
		if errors.Is(err, schema.ResourceSchemaNotFound) || errors.Is(err, schema.ReporterSchemaNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// commonShallowValidate validates the common representation for a given resourceType.
func (sc *SchemaUsecase) commonShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}) error {
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

// reporterShallowValidate validates the specific reporter representation for a given resourceType/reporterType.
func (sc *SchemaUsecase) reporterShallowValidate(ctx context.Context, resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error {
	reporter, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

	// Case 1: No schema found for resourceType:reporterType
	if reporter.ValidationSchema == nil {
		if len(reporterRepresentation) > 0 {
			return fmt.Errorf("no schema found for '%s:%s', but reporter representation was provided. Submission is not allowed", resourceType, reporterType)
		}
		sc.Log.Debugf("no schema found for %s:%s, treating as abstract reporter representation", resourceType, reporterType)
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
