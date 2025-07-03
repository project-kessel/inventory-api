package usecase

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	v1beta2model "github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// ResourceUsecase handles v1beta2 resource operations with a simplified approach
type ResourceUsecase struct {
	CommonRepresentationRepository   v1beta2model.CommonRepresentationRepository
	ReporterRepresentationRepository v1beta2model.ReporterRepresentationRepository
	ResourceWithReferencesRepository v1beta2model.ResourceWithReferencesRepository
	DB                               *gorm.DB
	TransactionManager               TransactionManager
	MetricsCollector                 *metricscollector.MetricsCollector
	Namespace                        string
	Log                              *log.Helper
}

// NewResourceUsecase creates a new v1beta2 usecase with minimal dependencies
func NewResourceUsecase(
	commonRepresentationRepository v1beta2model.CommonRepresentationRepository,
	reporterRepresentationRepository v1beta2model.ReporterRepresentationRepository,
	resourceWithReferencesRepository v1beta2model.ResourceWithReferencesRepository,
	database *gorm.DB,
	transactionManager TransactionManager,
	metricsCollector *metricscollector.MetricsCollector,
	namespace string,
	logger log.Logger,
) *ResourceUsecase {
	return &ResourceUsecase{
		CommonRepresentationRepository:   commonRepresentationRepository,
		ReporterRepresentationRepository: reporterRepresentationRepository,
		ResourceWithReferencesRepository: resourceWithReferencesRepository,
		DB:                               database,
		TransactionManager:               transactionManager,
		MetricsCollector:                 metricsCollector,
		Namespace:                        namespace,
		Log:                              log.NewHelper(logger),
	}
}

// ReportResource implements the v1beta2 upsert algorithm with a simplified approach
func (usecase *ResourceUsecase) ReportResource(ctx context.Context, request *v1beta2.ReportResourceRequest) error {
	usecase.Log.WithContext(ctx).Info("Starting upsert for resource type: ", request.GetType())

	// Use serializable transaction for consistency
	err := usecase.TransactionManager.ExecuteInSerializableTransaction(usecase.DB, func(transaction *gorm.DB) error {
		// Generate transaction ID for outbox events
		transactionId, err := uuid.NewV7()
		if err != nil {
			return err
		}
		transactionIdString := transactionId.String()

		// Create reporter representation ID for lookup
		reporterRepresentationId := v1beta2model.ReporterRepresentationId{
			LocalResourceID:    request.GetRepresentations().GetMetadata().GetLocalResourceId(),
			ReporterType:       request.GetReporterType(),
			ResourceType:       request.GetType(),
			ReporterInstanceID: request.GetReporterInstanceId(),
		}

		// Check if resource already exists
		existingReferences, err := usecase.ResourceWithReferencesRepository.FindAllReferencesByReporterRepresentationId(ctx, reporterRepresentationId)
		if err != nil {
			return err
		}

		if len(existingReferences) == 0 {
			// Create new resource
			err = usecase.createNewResource(ctx, transaction, request, transactionIdString)
			if err != nil {
				return err
			}
		} else {
			// Update existing resource
			err = usecase.updateExistingResource(ctx, transaction, request, existingReferences, transactionIdString)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		usecase.Log.WithContext(ctx).Errorf("Failed to upsert resource: %v", err)
		return err
	}

	usecase.Log.WithContext(ctx).Info("Successfully upserted resource")
	return nil
}

// createNewResource handles the creation scenario when no existing references are found
func (usecase *ResourceUsecase) createNewResource(ctx context.Context, transaction *gorm.DB, request *v1beta2.ReportResourceRequest, transactionIdString string) error {
	// Generate new UUID for resource
	resourceId, err := uuid.NewV7()
	cv := 1
	if err != nil {
		return err
	}

	// Create Resource entity
	resource := &v1beta2model.Resource{
		ID:   resourceId,
		Type: request.GetType(),
	}

	// Create CommonRepresentation
	commonRepresentation := &v1beta2model.CommonRepresentation{
		BaseRepresentation: v1beta2model.BaseRepresentation{
			Data: request.GetRepresentations().GetCommon().AsMap(),
		},
		LocalResourceID: resourceId.String(),
		ReporterType:    "inventory",
		ResourceType:    request.GetType(),
		Version:         1,
		ReportedBy:      request.GetReporterType() + "/" + request.GetReporterInstanceId(),
	}

	// Create RepresentationReference for Common Representation
	commonReference := &v1beta2model.RepresentationReference{
		ResourceID:            resourceId,
		LocalResourceID:       resourceId.String(),
		ReporterType:          "inventory",
		ResourceType:          request.GetType(),
		RepresentationVersion: 1,
		Generation:            1,
		Tombstone:             false,
	}

	// Create ReporterRepresentation
	reporterRepresentation := &v1beta2model.ReporterRepresentation{
		BaseRepresentation: v1beta2model.BaseRepresentation{
			Data: request.GetRepresentations().GetReporter().AsMap(),
		},
		LocalResourceID:    request.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:       request.GetReporterType(),
		ResourceType:       request.GetType(),
		Version:            1,
		ReporterInstanceID: request.GetReporterInstanceId(),
		Generation:         1,
		APIHref:            request.GetRepresentations().GetMetadata().GetApiHref(),
		ConsoleHref:        request.GetRepresentations().GetMetadata().GetConsoleHref(),
		CommonVersion:      &cv,
		Tombstone:          false,
		ReporterVersion:    request.GetRepresentations().GetMetadata().GetReporterVersion(),
	}

	// Create RepresentationReference for Reporter Representation
	reporterReference := &v1beta2model.RepresentationReference{
		ResourceID:            resourceId,
		LocalResourceID:       request.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:          request.GetReporterType(),
		ResourceType:          request.GetType(),
		ReporterInstanceID:    request.GetReporterInstanceId(),
		RepresentationVersion: 1,
		Generation:            1,
		Tombstone:             false,
	}

	// Create ResourceWithReferences aggregate
	resourceWithReferences := &v1beta2model.ResourceWithReferences{
		Resource:                 resource,
		RepresentationReferences: []*v1beta2model.RepresentationReference{commonReference, reporterReference},
	}

	_, err = usecase.ResourceWithReferencesRepository.Create(ctx, resourceWithReferences)
	if err != nil {
		return err
	}

	// Persist entities using regular Create methods (transaction is handled by TransactionManager)
	_, err = usecase.CommonRepresentationRepository.Create(ctx, commonRepresentation)
	if err != nil {
		return err
	}

	_, err = usecase.ReporterRepresentationRepository.Create(ctx, reporterRepresentation)
	if err != nil {
		return err
	}

	// Handle outbox events
	err = usecase.publishOutboxEvents(transaction, resource, commonRepresentation, reporterRepresentation, model.OperationTypeCreated, transactionIdString)
	if err != nil {
		return err
	}

	// Update metrics
	if usecase.MetricsCollector != nil {
		metricscollector.Incr(usecase.MetricsCollector.OutboxEventWrites, string(model.OperationTypeCreated), nil)
	}

	return nil
}

// updateExistingResource handles the update scenario when existing references are found
func (usecase *ResourceUsecase) updateExistingResource(ctx context.Context, transaction *gorm.DB, request *v1beta2.ReportResourceRequest, existingReferences []*v1beta2model.RepresentationReference, transactionIdString string) error {
	// Get the resource ID from the first reference (all references should have the same resource_id)
	resourceID := existingReferences[0].ResourceID

	// Determine current versions for common and reporter representations
	var currentCommonVersion int
	var currentReporterVersion int

	for _, ref := range existingReferences {
		if ref.ReporterType == "inventory" {
			currentCommonVersion = ref.RepresentationVersion
		} else if ref.ReporterType == request.GetReporterType() {
			currentReporterVersion = ref.RepresentationVersion
		}
	}

	// Variables to track what we create for outbox events
	var updatedCommonRepresentation *v1beta2model.CommonRepresentation
	var updatedReporterRepresentation *v1beta2model.ReporterRepresentation

	// Handle common representation update if provided
	if request.GetRepresentations().GetCommon() != nil {
		newCommonVersion := currentCommonVersion + 1

		// Create new CommonRepresentation with incremented version
		updatedCommonRepresentation = &v1beta2model.CommonRepresentation{
			BaseRepresentation: v1beta2model.BaseRepresentation{
				Data: request.GetRepresentations().GetCommon().AsMap(),
			},
			LocalResourceID: resourceID.String(),
			ReporterType:    "inventory",
			ResourceType:    request.GetType(),
			Version:         newCommonVersion,
			ReportedBy:      request.GetReporterType() + "/" + request.GetReporterInstanceId(),
		}

		// Persist the new common representation
		_, err := usecase.CommonRepresentationRepository.Create(ctx, updatedCommonRepresentation)
		if err != nil {
			return err
		}

		// Update the representation_version in RepresentationReference for inventory
		_, err = usecase.ResourceWithReferencesRepository.UpdateCommonRepresentationVersion(ctx, resourceID, newCommonVersion)
		if err != nil {
			return err
		}
	}

	// Handle reporter representation update if provided
	if request.GetRepresentations().GetReporter() != nil {
		newReporterVersion := currentReporterVersion + 1

		// Find the generation for this reporter
		var generation int
		for _, ref := range existingReferences {
			if ref.ReporterType == request.GetReporterType() {
				generation = ref.Generation
				break
			}
		}

		// Determine the common version to use - only set if common was updated in this request
		var commonVersionPtr *int
		if updatedCommonRepresentation != nil {
			commonVersionPtr = &updatedCommonRepresentation.Version
		}

		// Create new ReporterRepresentation with incremented version
		updatedReporterRepresentation = &v1beta2model.ReporterRepresentation{
			BaseRepresentation: v1beta2model.BaseRepresentation{
				Data: request.GetRepresentations().GetReporter().AsMap(),
			},
			LocalResourceID:    request.GetRepresentations().GetMetadata().GetLocalResourceId(),
			ReporterType:       request.GetReporterType(),
			ResourceType:       request.GetType(),
			Version:            newReporterVersion,
			ReporterInstanceID: request.GetReporterInstanceId(),
			Generation:         generation,
			APIHref:            request.GetRepresentations().GetMetadata().GetApiHref(),
			ConsoleHref:        request.GetRepresentations().GetMetadata().GetConsoleHref(),
			CommonVersion:      commonVersionPtr, // Use updated common version if common was updated, otherwise nil
			Tombstone:          false,
			ReporterVersion:    request.GetRepresentations().GetMetadata().GetReporterVersion(),
		}

		// Persist the new reporter representation
		_, err := usecase.ReporterRepresentationRepository.Create(ctx, updatedReporterRepresentation)
		if err != nil {
			return err
		}

		// Update the representation_version in RepresentationReference for this reporter
		_, err = usecase.ResourceWithReferencesRepository.UpdateReporterRepresentationVersion(ctx, resourceID, request.GetReporterType(), request.GetRepresentations().GetMetadata().GetLocalResourceId(), newReporterVersion)
		if err != nil {
			return err
		}
	}

	//TODO: What outbox events to publish for updates
	// 1. Tuple updates -> so if common_representation is updated (for now) or just current state?
	// 2. Resource updates -> Do we publish the latest version of the Resource+representations?

	// Load the resource for outbox events (we only need this one query)
	var resource v1beta2model.Resource
	// TODO: Fix this to work with fake repositories - for now use the resourceID from references
	resource.ID = resourceID
	resource.Type = request.GetType()

	// For outbox events, we need to handle the case where only some representations were updated
	// If no common representation was updated, we need to get the latest one
	if updatedCommonRepresentation == nil {
		// TODO: Fix this to work with fake repositories
		// For now, create a placeholder common representation
		updatedCommonRepresentation = &v1beta2model.CommonRepresentation{
			BaseRepresentation: v1beta2model.BaseRepresentation{
				Data: request.GetRepresentations().GetCommon().AsMap(),
			},
			LocalResourceID: resourceID.String(),
			ReporterType:    "inventory",
			ResourceType:    request.GetType(),
			Version:         currentCommonVersion,
			ReportedBy:      request.GetReporterType() + "/" + request.GetReporterInstanceId(),
		}
	}

	// If no reporter representation was updated, we need to get the latest one
	if updatedReporterRepresentation == nil {
		// TODO: Fix this to work with fake repositories
		// For now, create a placeholder reporter representation
		updatedReporterRepresentation = &v1beta2model.ReporterRepresentation{
			BaseRepresentation: v1beta2model.BaseRepresentation{
				Data: request.GetRepresentations().GetReporter().AsMap(),
			},
			LocalResourceID:    request.GetRepresentations().GetMetadata().GetLocalResourceId(),
			ReporterType:       request.GetReporterType(),
			ResourceType:       request.GetType(),
			Version:            currentReporterVersion,
			ReporterInstanceID: request.GetReporterInstanceId(),
			Generation:         1, // TODO: Get from existing references
			APIHref:            request.GetRepresentations().GetMetadata().GetApiHref(),
			ConsoleHref:        request.GetRepresentations().GetMetadata().GetConsoleHref(),
			CommonVersion:      nil,
			Tombstone:          false,
			ReporterVersion:    request.GetRepresentations().GetMetadata().GetReporterVersion(),
		}
	}

	// Handle outbox events for update using the representations we have (either newly created or loaded)
	err := usecase.publishOutboxEvents(transaction, &resource, updatedCommonRepresentation, updatedReporterRepresentation, model.OperationTypeUpdated, transactionIdString)
	if err != nil {
		return err
	}

	// Update metrics
	if usecase.MetricsCollector != nil {
		metricscollector.Incr(usecase.MetricsCollector.OutboxEventWrites, string(model.OperationTypeUpdated), nil)
	}

	return nil
}

// publishOutboxEvents creates and publishes outbox events for the resource
func (usecase *ResourceUsecase) publishOutboxEvents(transaction *gorm.DB, resource *v1beta2model.Resource, commonRepresentation *v1beta2model.CommonRepresentation, reporterRepresentation *v1beta2model.ReporterRepresentation, operationType model.EventOperationType, transactionId string) error {
	// Skip outbox events if transaction is nil (e.g., in tests with fake transaction manager)
	if transaction == nil {
		usecase.Log.Debug("Skipping outbox events - no transaction provided")
		return nil
	}

	// Convert to legacy format for outbox events
	legacyResource := usecase.convertToLegacyResource(resource, commonRepresentation, reporterRepresentation)

	resourceMessage, tupleMessage, err := model.NewOutboxEventsFromResource(*legacyResource, usecase.Namespace, operationType, transactionId)
	if err != nil {
		return err
	}

	err = data.PublishOutboxEvent(transaction, resourceMessage)
	if err != nil {
		return err
	}

	err = data.PublishOutboxEvent(transaction, tupleMessage)
	if err != nil {
		return err
	}

	return nil
}

// convertToLegacyResource converts v1beta2 models to legacy model.Resource
func (usecase *ResourceUsecase) convertToLegacyResource(resource *v1beta2model.Resource, commonRepresentation *v1beta2model.CommonRepresentation, reporterRepresentation *v1beta2model.ReporterRepresentation) *model.Resource {
	// Extract basic fields from common representation data
	var organizationId, workspaceId string
	var labels model.Labels

	if commonRepresentation.Data != nil {
		if organization, ok := commonRepresentation.Data["org_id"].(string); ok {
			organizationId = organization
		}
		if workspace, ok := commonRepresentation.Data["workspace_id"].(string); ok {
			workspaceId = workspace
		}
		// Handle labels extraction if needed
		if labelsData, ok := commonRepresentation.Data["labels"].([]interface{}); ok {
			labels = make(model.Labels, len(labelsData))
			for i, labelInterface := range labelsData {
				if labelMap, ok := labelInterface.(map[string]interface{}); ok {
					if key, keyOk := labelMap["key"].(string); keyOk {
						if value, valueOk := labelMap["value"].(string); valueOk {
							labels[i] = model.Label{Key: key, Value: value}
						}
					}
				}
			}
		}
	}

	// Use the most recent timestamp from either representation
	var createdAt, updatedAt *time.Time
	if !commonRepresentation.CreatedAt.IsZero() {
		createdAt = &commonRepresentation.CreatedAt
	}
	if !commonRepresentation.UpdatedAt.IsZero() {
		updatedAt = &commonRepresentation.UpdatedAt
	}

	// If reporter representation has more recent timestamps, use those
	if !reporterRepresentation.CreatedAt.IsZero() && (createdAt == nil || reporterRepresentation.CreatedAt.Before(*createdAt)) {
		createdAt = &reporterRepresentation.CreatedAt
	}
	if !reporterRepresentation.UpdatedAt.IsZero() && (updatedAt == nil || reporterRepresentation.UpdatedAt.After(*updatedAt)) {
		updatedAt = &reporterRepresentation.UpdatedAt
	}

	return &model.Resource{
		ID:                 resource.ID,
		ResourceType:       resource.Type,
		OrgId:              organizationId,
		WorkspaceId:        workspaceId,
		ResourceData:       reporterRepresentation.Data,
		Labels:             labels,
		ReporterId:         reporterRepresentation.ReporterInstanceID,
		ReporterResourceId: reporterRepresentation.LocalResourceID,
		ConsoleHref:        reporterRepresentation.ConsoleHref,
		ApiHref:            reporterRepresentation.APIHref,
		ReporterType:       reporterRepresentation.ReporterType,
		ReporterVersion:    reporterRepresentation.ReporterVersion,
		InventoryId:        &resource.ID, // In v1beta2, Resource.ID is the inventory ID
		CreatedAt:          createdAt,
		UpdatedAt:          updatedAt,
	}
}
