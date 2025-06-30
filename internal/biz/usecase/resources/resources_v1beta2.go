package resources

import (
	"context"
	"database/sql"
	"errors"

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
	commonRepRepo modelsv1beta2.CommonRepresentationRepository,
	reporterRepRepo modelsv1beta2.ReporterRepresentationRepository,
	resourceRepo modelsv1beta2.ResourceWithReferencesRepository,
	db *gorm.DB,
	metricsCollector *metricscollector.MetricsCollector,
	namespace string,
	logger log.Logger,
) *ResourceUsecase {
	return &ResourceUsecase{
		CommonRepresentationRepository:   commonRepRepo,
		ReporterRepresentationRepository: reporterRepRepo,
		ResourceWithReferencesRepository: resourceRepo,
		DB:                               db,
		MetricsCollector:                 metricsCollector,
		Namespace:                        namespace,
		Log:                              log.NewHelper(logger),
	}
}

// UpsertResource implements the v1beta2 upsert algorithm with a simplified approach
func (uc *ResourceUsecase) UpsertResource(ctx context.Context, req *v1beta2.ReportResourceRequest) (*model.Resource, error) {
	uc.Log.WithContext(ctx).Info("Starting v1beta2 upsert for resource type: ", req.GetType())

	var result *model.Resource

	// Use serializable transaction for consistency
	err := uc.DB.Transaction(func(tx *gorm.DB) error {
		// Generate transaction ID for outbox events
		txid, err := uuid.NewV7()
		if err != nil {
			return err
		}
		txidStr := txid.String()

		// Create reporter representation ID for lookup
		reporterId := modelsv1beta2.ReporterRepresentationId{
			LocalResourceID:    req.GetRepresentations().GetMetadata().GetLocalResourceId(),
			ReporterType:       req.GetReporterType(),
			ResourceType:       req.GetType(),
			ReporterInstanceID: req.GetReporterInstanceId(),
		}

		// Check if resource already exists
		existingRefs, err := uc.ResourceWithReferencesRepository.FindAllReferencesByReporterRepresentationId(ctx, reporterId)
		if err != nil {
			return err
		}

		if len(existingRefs) == 0 {
			// Create new resource
			result, err = uc.createNewResource(ctx, tx, req, txidStr)
			if err != nil {
				return err
			}
		} else {
			// Update existing resource
			result, err = uc.updateExistingResource(ctx, tx, req, existingRefs, txidStr)
			if err != nil {
				return err
			}
		}

		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})

	if err != nil {
		uc.Log.WithContext(ctx).Errorf("Failed to upsert resource: %v", err)
		return nil, err
	}

	uc.Log.WithContext(ctx).Infof("Successfully upserted resource: %v", result.ID)
	return result, nil
}

// createNewResource handles the creation scenario when no existing references are found
func (uc *ResourceUsecase) createNewResource(ctx context.Context, tx *gorm.DB, req *v1beta2.ReportResourceRequest, txidStr string) (*model.Resource, error) {
	// Generate new UUID for resource
	resourceId, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	// Create Resource entity
	resource := &modelsv1beta2.Resource{
		ID:   resourceId,
		Type: req.GetType(),
	}

	// Create CommonRepresentation
	commonRep := &modelsv1beta2.CommonRepresentation{
		BaseRepresentation: modelsv1beta2.BaseRepresentation{
			Data: req.GetRepresentations().GetCommon().AsMap(),
		},
		LocalResourceID: req.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:    req.GetReporterType(),
		ResourceType:    req.GetType(),
		Version:         1,
		ReportedBy:      req.GetReporterInstanceId(),
	}

	// Create ReporterRepresentation
	reporterRep := &modelsv1beta2.ReporterRepresentation{
		BaseRepresentation: modelsv1beta2.BaseRepresentation{
			Data: req.GetRepresentations().GetReporter().AsMap(),
		},
		LocalResourceID:    req.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:       req.GetReporterType(),
		ResourceType:       req.GetType(),
		Version:            1,
		ReporterInstanceID: req.GetReporterInstanceId(),
		Generation:         0,
		APIHref:            req.GetRepresentations().GetMetadata().GetApiHref(),
		ConsoleHref:        req.GetRepresentations().GetMetadata().GetConsoleHref(),
		CommonVersion:      1,
		Tombstone:          false,
		ReporterVersion:    req.GetRepresentations().GetMetadata().GetReporterVersion(),
	}

	// Create RepresentationReference entries
	commonRef := &modelsv1beta2.RepresentationReference{
		ResourceID:            resourceId,
		LocalResourceID:       req.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:          req.GetReporterType(),
		ResourceType:          req.GetType(),
		ReporterInstanceID:    req.GetReporterInstanceId(),
		RepresentationVersion: 1,
		Generation:            0,
		Tombstone:             false,
	}

	reporterRef := &modelsv1beta2.RepresentationReference{
		ResourceID:            resourceId,
		LocalResourceID:       req.GetRepresentations().GetMetadata().GetLocalResourceId(),
		ReporterType:          req.GetReporterType(),
		ResourceType:          req.GetType(),
		ReporterInstanceID:    req.GetReporterInstanceId(),
		RepresentationVersion: 1,
		Generation:            0,
		Tombstone:             false,
	}

	// Create ResourceWithReferences aggregate
	resourceWithRefs := &modelsv1beta2.ResourceWithReferences{
		Resource:                 resource,
		RepresentationReferences: []*modelsv1beta2.RepresentationReference{commonRef, reporterRef},
	}

	// Persist entities using WithTx methods
	commonRepository := uc.CommonRepresentationRepository.(*datav1beta2.CommonRepresentationRepository)
	_, err = commonRepository.CreateWithTx(ctx, tx, commonRep)
	if err != nil {
		return nil, err
	}

	reporterRepository := uc.ReporterRepresentationRepository.(*datav1beta2.ReporterRepresentationRepository)
	_, err = reporterRepository.CreateWithTx(ctx, tx, reporterRep)
	if err != nil {
		return nil, err
	}

	resourceRepository := uc.ResourceWithReferencesRepository.(*datav1beta2.ResourceWithReferencesRepository)
	_, err = resourceRepository.CreateWithTx(ctx, tx, resourceWithRefs)
	if err != nil {
		return nil, err
	}

	// Handle outbox events
	err = uc.publishOutboxEvents(tx, resource, commonRep, reporterRep, model.OperationTypeCreated, txidStr)
	if err != nil {
		return nil, err
	}

	// Update metrics
	metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(model.OperationTypeCreated), nil)

	// Convert to legacy Resource format for return
	legacyResource := uc.convertToLegacyResource(resource, commonRep, reporterRep)
	return legacyResource, nil
}

// updateExistingResource handles the update scenario when existing references are found
func (uc *ResourceUsecase) updateExistingResource(ctx context.Context, tx *gorm.DB, req *v1beta2.ReportResourceRequest, existingRefs []*modelsv1beta2.RepresentationReference, txidStr string) (*model.Resource, error) {
	// For now, return an error - we'll implement this later
	return nil, errors.New("update scenario not yet implemented")
}

// publishOutboxEvents creates and publishes outbox events for the resource
func (uc *ResourceUsecase) publishOutboxEvents(tx *gorm.DB, resource *modelsv1beta2.Resource, commonRep *modelsv1beta2.CommonRepresentation, reporterRep *modelsv1beta2.ReporterRepresentation, operationType model.EventOperationType, txid string) error {
	// Convert to legacy format for outbox events
	legacyResource := uc.convertToLegacyResource(resource, commonRep, reporterRep)

	resourceMessage, tupleMessage, err := model.NewOutboxEventsFromResource(*legacyResource, uc.Namespace, operationType, txid)
	if err != nil {
		return err
	}

	err = data.PublishOutboxEvent(tx, resourceMessage)
	if err != nil {
		return err
	}

	err = data.PublishOutboxEvent(tx, tupleMessage)
	if err != nil {
		return err
	}

	return nil
}

// convertToLegacyResource converts v1beta2 models to legacy model.Resource
func (uc *ResourceUsecase) convertToLegacyResource(resource *modelsv1beta2.Resource, commonRep *modelsv1beta2.CommonRepresentation, reporterRep *modelsv1beta2.ReporterRepresentation) *model.Resource {
	// Extract basic fields from common representation data
	var orgId, workspaceId string
	var resourceData model.JsonObject
	var labels model.Labels

	if commonRep.Data != nil {
		if org, ok := commonRep.Data["org_id"].(string); ok {
			orgId = org
		}
		if workspace, ok := commonRep.Data["workspace_id"].(string); ok {
			workspaceId = workspace
		}
		if data, ok := commonRep.Data["resource_data"].(model.JsonObject); ok {
			resourceData = data
		}
		// Handle labels extraction if needed
		if labelsData, ok := commonRep.Data["labels"].([]interface{}); ok {
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

	return &model.Resource{
		ID:                 resource.ID,
		ResourceType:       resource.Type,
		OrgId:              orgId,
		WorkspaceId:        workspaceId,
		ResourceData:       resourceData,
		Labels:             labels,
		ReporterId:         reporterRep.ReporterInstanceID,
		ReporterResourceId: reporterRep.LocalResourceID,
		ConsoleHref:        reporterRep.ConsoleHref,
		ApiHref:            reporterRep.APIHref,
		ReporterType:       reporterRep.ReporterType,
		InventoryId:        &resource.ID, // In v1beta2, Resource.ID is the inventory ID
	}
}
