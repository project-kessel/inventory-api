package resources

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	modelsv1beta2 "github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
	"github.com/project-kessel/inventory-api/internal/data"
	datav1beta2 "github.com/project-kessel/inventory-api/internal/data/v1beta2"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

// ResourceUsecase handles v1beta2 resource operations with a simplified approach
type ResourceUsecase struct {
	CommonRepresentationRepository   modelsv1beta2.CommonRepresentationRepository
	ReporterRepresentationRepository modelsv1beta2.ReporterRepresentationRepository
	ResourceWithReferencesRepository modelsv1beta2.ResourceWithReferencesRepository
	DB                               *gorm.DB
	MetricsCollector                 *metricscollector.MetricsCollector
	Namespace                        string
	Log                              *log.Helper
}

// NewResourceUsecase creates a new v1beta2 usecase with minimal dependencies
func NewResourceUsecase(
	commonRepresentationRepository modelsv1beta2.CommonRepresentationRepository,
	reporterRepresentationRepository modelsv1beta2.ReporterRepresentationRepository,
	resourceWithReferencesRepository modelsv1beta2.ResourceWithReferencesRepository,
	database *gorm.DB,
	metricsCollector *metricscollector.MetricsCollector,
	namespace string,
	logger log.Logger,
) *ResourceUsecase {
	return &ResourceUsecase{
		CommonRepresentationRepository:   commonRepresentationRepository,
		ReporterRepresentationRepository: reporterRepresentationRepository,
		ResourceWithReferencesRepository: resourceWithReferencesRepository,
		DB:                               database,
		MetricsCollector:                 metricsCollector,
		Namespace:                        namespace,
		Log:                              log.NewHelper(logger),
	}
}

// UpsertResource implements the v1beta2 upsert algorithm with a simplified approach
func (usecase *ResourceUsecase) UpsertResource(ctx context.Context, request *v1beta2.ReportResourceRequest) error {
	usecase.Log.WithContext(ctx).Info("Starting v1beta2 upsert for resource type: ", request.GetType())

	// Use serializable transaction for consistency
	err := usecase.DB.Transaction(func(transaction *gorm.DB) error {
		// Generate transaction ID for outbox events
		transactionId, err := uuid.NewV7()
		if err != nil {
			return err
		}
		transactionIdString := transactionId.String()

		// Create reporter representation ID for lookup
		reporterRepresentationId := modelsv1beta2.ReporterRepresentationId{
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
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})

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
	if err != nil {
		return err
	}

	// Create Resource entity
	resource := &modelsv1beta2.Resource{
		ID:   resourceId,
		Type: request.GetType(),
	}

	// Create CommonRepresentation
	commonRepresentation := &modelsv1beta2.CommonRepresentation{
		BaseRepresentation: modelsv1beta2.BaseRepresentation{
			Data: request.GetRepresentations().GetCommon().AsMap(),
		},
		LocalResourceID: resourceId.String(),
		ReporterType:    "inventory",
		ResourceType:    request.GetType(),
		Version:         1,
		ReportedBy:      request.GetReporterType() + "/" + request.GetReporterInstanceId(),
	}

	// Create RepresentationReference for Common Representation
	commonReference := &modelsv1beta2.RepresentationReference{
		ResourceID:            resourceId,
		LocalResourceID:       resourceId.String(),
		ReporterType:          "inventory",
		ResourceType:          request.GetType(),
		RepresentationVersion: 1,
		Generation:            1,
		Tombstone:             false,
	}

	// Create ReporterRepresentation
	reporterRepresentation := &modelsv1beta2.ReporterRepresentation{
		BaseRepresentation: modelsv1beta2.BaseRepresentation{
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
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    request.GetRepresentations().GetMetadata().GetReporterVersion(),
	}

	// Create RepresentationReference for Reporter Representation
	reporterReference := &modelsv1beta2.RepresentationReference{
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
	resourceWithReferences := &modelsv1beta2.ResourceWithReferences{
		Resource:                 resource,
		RepresentationReferences: []*modelsv1beta2.RepresentationReference{commonReference, reporterReference},
	}

	resourceRepository := usecase.ResourceWithReferencesRepository.(*datav1beta2.ResourceWithReferencesRepository)
	_, err = resourceRepository.CreateWithTx(ctx, transaction, resourceWithReferences)
	if err != nil {
		return err
	}

	// Persist entities using WithTx methods
	commonRepository := usecase.CommonRepresentationRepository.(*datav1beta2.CommonRepresentationRepository)
	_, err = commonRepository.CreateWithTx(ctx, transaction, commonRepresentation)
	if err != nil {
		return err
	}

	reporterRepository := usecase.ReporterRepresentationRepository.(*datav1beta2.ReporterRepresentationRepository)
	_, err = reporterRepository.CreateWithTx(ctx, transaction, reporterRepresentation)
	if err != nil {
		return err
	}

	// Handle outbox events
	err = usecase.publishOutboxEvents(transaction, resource, commonRepresentation, reporterRepresentation, model.OperationTypeCreated, transactionIdString)
	if err != nil {
		return err
	}

	// Update metrics
	metricscollector.Incr(usecase.MetricsCollector.OutboxEventWrites, string(model.OperationTypeCreated), nil)

	return nil
}

// updateExistingResource handles the update scenario when existing references are found
func (usecase *ResourceUsecase) updateExistingResource(ctx context.Context, transaction *gorm.DB, request *v1beta2.ReportResourceRequest, existingReferences []*modelsv1beta2.RepresentationReference, transactionIdString string) error {
	// For now, return an error - we'll implement this later
	return errors.New("update scenario not yet implemented")
}

// publishOutboxEvents creates and publishes outbox events for the resource
func (usecase *ResourceUsecase) publishOutboxEvents(transaction *gorm.DB, resource *modelsv1beta2.Resource, commonRepresentation *modelsv1beta2.CommonRepresentation, reporterRepresentation *modelsv1beta2.ReporterRepresentation, operationType model.EventOperationType, transactionId string) error {
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
func (usecase *ResourceUsecase) convertToLegacyResource(resource *modelsv1beta2.Resource, commonRepresentation *modelsv1beta2.CommonRepresentation, reporterRepresentation *modelsv1beta2.ReporterRepresentation) *model.Resource {
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
