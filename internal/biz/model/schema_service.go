package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
)

// SchemaService is a domain service that orchestrates schema-based operations
// such as validation and reporter verification.
type SchemaService struct {
	schemaRepository SchemaRepository
}

// NewSchemaService creates a new SchemaService with the given repository.
func NewSchemaService(schemaRepository SchemaRepository) *SchemaService {
	return &SchemaService{
		schemaRepository: schemaRepository,
	}
}

func (sc *SchemaService) CalculateTuples(currentRepresentation, previousRepresentation *Representations, key ReporterResourceKey) (TuplesToReplicate, error) {
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
		return TuplesToReplicate{}, nil
	}

	// Build tuples to create and delete
	var tuplesToCreate, tuplesToDelete []RelationsTuple

	if currentWorkspaceID != "" {
		tuplesToCreate = append(tuplesToCreate, NewWorkspaceRelationsTuple(currentWorkspaceID, key))
	}

	if previousWorkspaceID != "" {
		tuplesToDelete = append(tuplesToDelete, NewWorkspaceRelationsTuple(previousWorkspaceID, key))
	}

	return NewTuplesToReplicate(tuplesToCreate, tuplesToDelete)
}

// IsReporterForResource validates the resourceType and reporterType combination is valid.
// Returns true if there is a reporter that reports said resource, false otherwise.
func (sc *SchemaService) IsReporterForResource(ctx context.Context, resourceType ResourceType, reporterType ReporterType) (bool, error) {
	if _, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType); err != nil {
		if errors.Is(err, ResourceSchemaNotFound) || errors.Is(err, ReporterSchemaNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// CommonShallowValidate validates the common representation for a given resourceType.
func (sc *SchemaService) CommonShallowValidate(ctx context.Context, resourceType ResourceType, commonRepresentation map[string]interface{}) error {
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
func (sc *SchemaService) ReporterShallowValidate(ctx context.Context, resourceType ResourceType, reporterType ReporterType, reporterRepresentation map[string]interface{}) error {
	reporter, err := sc.schemaRepository.GetReporterSchema(ctx, resourceType, reporterType)
	if err != nil {
		return err
	}

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
