package resources

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
)

type Repo struct {
	DB               *gorm.DB
	MetricsCollector *metricscollector.MetricsCollector
}

func New(db *gorm.DB, mc *metricscollector.MetricsCollector) *Repo {
	return &Repo{
		DB:               db,
		MetricsCollector: mc,
	}
}

func copyHistory(m *model.Resource, id uuid.UUID, operationType model.OperationType) *model.ResourceHistory {
	return &model.ResourceHistory{
		OrgId:         m.OrgId,
		ResourceData:  m.ResourceData,
		ResourceType:  m.ResourceType,
		WorkspaceId:   m.WorkspaceId,
		Reporter:      m.Reporter, //nolint:staticcheck
		ConsoleHref:   m.ConsoleHref,
		ApiHref:       m.ApiHref,
		Labels:        m.Labels,
		ResourceId:    id,
		OperationType: operationType,
	}
}

func (r *Repo) Create(ctx context.Context, m *model.Resource, namespace string, txid string) (*model.Resource, error) {
	db := r.DB.Session(&gorm.Session{})
	tx := db.Begin()
	updatedResources := []*model.Resource{}

	if m.InventoryId == nil {
		// New inventory resource
		inventoryResource := model.InventoryResource{
			ResourceType: m.ResourceType,
			WorkspaceId:  m.WorkspaceId,
		}
		// Create a new inventory resource
		if err := tx.Create(&inventoryResource).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("creating inventory resource: %w", err)
		}
		m.InventoryId = &inventoryResource.ID
	}

	if err := tx.Create(m).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Create(copyHistory(m, m.ID, model.OperationTypeCreate)).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Handle workspace updates for other resources with the same inventory ID
	updatedResources, err := r.handleWorkspaceUpdates(tx, m, updatedResources)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Deprecated
	// TODO: Remove this when all resources are created with inventory ID
	if err := tx.Create(&model.LocalInventoryToResource{
		ResourceId:         m.ID,
		ReporterResourceId: model.ReporterResourceIdFromResource(m),
	}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Publish outbox events for primary resource
	err = handleOutboxEvents(tx, *m, namespace, model.OperationTypeCreated, txid)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Publish outbox events for other resources with the same inventory ID
	for _, updatedResource := range updatedResources {
		err = handleOutboxEvents(tx, *updatedResource, namespace, model.OperationTypeUpdated, "")
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model.OperationTypeCreated), nil)
	return m, nil
}

func (r *Repo) Update(ctx context.Context, m *model.Resource, id uuid.UUID, namespace string, txid string) (*model.Resource, error) {
	db := r.DB.Session(&gorm.Session{})
	updatedResources := []*model.Resource{}

	resource, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()
	if err := tx.Create(copyHistory(m, id, model.OperationTypeUpdate)).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	m.ID = id
	m.CreatedAt = resource.CreatedAt
	m.InventoryId = resource.InventoryId
	if err := tx.Save(m).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Handle workspace updates for other resources with the same inventory ID
	updatedResources, err = r.handleWorkspaceUpdates(tx, m, updatedResources)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Publish outbox events for primary resource
	err = handleOutboxEvents(tx, *m, namespace, model.OperationTypeUpdated, txid)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// Publish outbox event for the primary resource and other resources with the same inventory ID
	for _, updatedResource := range updatedResources {
		err = handleOutboxEvents(tx, *updatedResource, namespace, model.OperationTypeUpdated, "")
		if err != nil {
			tx.Rollback()
			return nil, err
		}
	}

	tx.Commit()
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model.OperationTypeUpdated), nil)
	return m, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model.Resource, error) {
	db := r.DB.Session(&gorm.Session{})

	resource, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	tx := db.Begin()
	if err := tx.Create(copyHistory(resource, resource.ID, model.OperationTypeDelete)).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	// Delete relationships - We don't yet care about keeping history of deleted relationships of a deleted resource.
	if err := tx.Where("subject_id = ? or object_id = ?", id, id).Delete(&model.Relationship{}).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Delete(resource).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if resource.InventoryId != nil {
		// Delete Inventory Resource if no other resources are referencing it
		var count int64
		if err := tx.Model(&model.Resource{}).Where("inventory_id = ?", *resource.InventoryId).Count(&count).Error; err != nil {
			tx.Rollback()
			return nil, err
		}
		if count == 0 {
			if err := tx.Delete(&model.InventoryResource{}, *resource.InventoryId).Error; err != nil {
				tx.Rollback()
				return nil, err
			}
		}
	}

	err = handleOutboxEvents(tx, *resource, namespace, model.OperationTypeDeleted, "")
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	tx.Commit()
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model.OperationTypeDeleted), nil)
	return resource, nil
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).First(&resource, id).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model.Resource, error) {
	session := r.DB.Session(&gorm.Session{})
	data := []*model.Resource{}

	log.Infof("FindByWorkspaceId: %s", workspace_id)
	if err := session.Where("workspace_id = ?", workspace_id).Find(&data).Error; err != nil {
		return nil, err
	}

	log.Infof("FindByWorkspaceId: data %+v", data)
	return data, nil
}

// Deprecated: Prefer FindByReporterData instead
func (r *Repo) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	session := r.DB.Session(&gorm.Session{})

	resourceId, err := data.GetLastResourceId(session, id)
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, resourceId)
}

func (r *Repo) FindByReporterResourceIdv1beta2(ctx context.Context, id model.ReporterResourceUniqueIndex) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model.ReporterResourceUniqueIndex{
		ReporterInstanceId: id.ReporterInstanceId,
		ReporterResourceId: id.ReporterResourceId,
		ResourceType:       id.ResourceType,
		ReporterType:       id.ReporterType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model.Resource{
		InventoryId:  inventoryId,
		ResourceType: resourceType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterInstanceId string, reporterType string) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model.Resource{
		InventoryId:        inventoryId,
		ReporterInstanceId: reporterInstanceId,
		ResourceType:       reporterType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByReporterData(ctx context.Context, reporterId string, reporterResourceId string) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model.Resource{
		ReporterId:         reporterId,
		ReporterResourceId: reporterResourceId,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) ListAll(context.Context) ([]*model.Resource, error) {
	var results []*model.Resource
	if err := r.DB.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func (r *Repo) handleWorkspaceUpdates(tx *gorm.DB, m *model.Resource, updatedResources []*model.Resource) ([]*model.Resource, error) {
	if m.InventoryId != nil {
		var inventoryResource model.InventoryResource
		if err := tx.First(&inventoryResource, m.InventoryId).Error; err != nil {
			return nil, fmt.Errorf("fetching inventory resource: %w", err)
		}
		// if workspace changes, update inventory resource
		if inventoryResource.WorkspaceId != m.WorkspaceId {
			inventoryResource.WorkspaceId = m.WorkspaceId
			if err := tx.Save(&inventoryResource).Error; err != nil {
				return nil, fmt.Errorf("updating inventory resource workspace ID: %w", err)
			}
			// get all resources with same inventory ID
			var resources []model.Resource
			if err := tx.Where("inventory_id = ?", m.InventoryId).Find(&resources).Error; err != nil {
				return nil, fmt.Errorf("fetching resources with inventory ID: %w", err)
			}
			// iterate all resources, update the workspace ID and save
			for _, resource := range resources {
				if m.ID == resource.ID {
					// skip the primary resource being updated
					continue
				}
				resource.WorkspaceId = m.WorkspaceId
				if err := tx.Save(&resource).Error; err != nil {
					return nil, fmt.Errorf("updating resource workspace ID: %w", err)
				}
				updatedResources = append(updatedResources, &resource)
			}
		}
	}
	return updatedResources, nil
}

func handleOutboxEvents(tx *gorm.DB, resource model.Resource, namespace string, operationType model.EventOperationType, txid string) error {
	resourceMessage, tupleMessage, err := model.NewOutboxEventsFromResource(resource, namespace, operationType, txid)
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
