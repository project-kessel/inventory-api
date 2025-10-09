package resources

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
	"github.com/project-kessel/inventory-api/internal/data"
)

type Repo struct {
	DB                 *gorm.DB
	MetricsCollector   *metricscollector.MetricsCollector
	TransactionManager usecase.TransactionManager
}

func New(db *gorm.DB, mc *metricscollector.MetricsCollector, transactionManager usecase.TransactionManager) *Repo {
	return &Repo{
		DB:                 db,
		MetricsCollector:   mc,
		TransactionManager: transactionManager,
	}
}

func copyHistory(m *model_legacy.Resource, id uuid.UUID, operationType model_legacy.OperationType) *model_legacy.ResourceHistory {
	return &model_legacy.ResourceHistory{
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

func (r *Repo) Create(ctx context.Context, m *model_legacy.Resource, namespace string, txid string) (*model_legacy.Resource, error) {
	if m == nil {
		return nil, fmt.Errorf("resource cannot be nil")
	}

	db := r.DB.Session(&gorm.Session{})
	var result *model_legacy.Resource
	err := r.TransactionManager.HandleSerializableTransaction("LegacyCreateResource", db, func(tx *gorm.DB) error {
		// Copy the resource to avoid modifying the original, necessary for serialized transaction retries
		resource := *m
		updatedResources := []*model_legacy.Resource{}

		if resource.InventoryId == nil {
			// New inventory resource
			inventoryResource := model_legacy.InventoryResource{
				ResourceType: resource.ResourceType,
				WorkspaceId:  resource.WorkspaceId,
			}
			// Create a new inventory resource
			if err := tx.Create(&inventoryResource).Error; err != nil {
				return fmt.Errorf("creating inventory resource: %w", err)
			}
			resource.InventoryId = &inventoryResource.ID
		}

		if err := tx.Create(&resource).Error; err != nil {
			return err
		}

		if err := tx.Create(copyHistory(&resource, resource.ID, model_legacy.OperationTypeCreate)).Error; err != nil {
			return err
		}

		// Handle workspace updates for other resources with the same inventory ID
		updatedResources, err := r.handleWorkspaceUpdates(tx, &resource, updatedResources)
		if err != nil {
			return err
		}

		// Deprecated
		// TODO: Remove this when all resources are created with inventory ID
		if err := tx.Create(&model_legacy.LocalInventoryToResource{
			ResourceId:         resource.ID,
			ReporterResourceId: model_legacy.ReporterResourceIdFromResource(&resource),
		}).Error; err != nil {
			return err
		}

		// Publish outbox events for primary resource
		err = handleOutboxEvents(tx, resource, namespace, model_legacy.OperationTypeCreated, txid)
		if err != nil {
			return err
		}

		// Publish outbox events for other resources with the same inventory ID
		for _, updatedResource := range updatedResources {
			err = handleOutboxEvents(tx, *updatedResource, namespace, model_legacy.OperationTypeUpdated, "")
			if err != nil {
				return err
			}
		}
		result = &resource
		return nil
	})
	if err != nil {
		return nil, err
	}
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model_legacy.OperationTypeCreated), nil)
	return result, nil
}

func (r *Repo) Update(ctx context.Context, m *model_legacy.Resource, id uuid.UUID, namespace string, txid string) (*model_legacy.Resource, error) {
	db := r.DB.Session(&gorm.Session{})
	var result *model_legacy.Resource
	err := r.TransactionManager.HandleSerializableTransaction("LegacyUpdateResource", db, func(tx *gorm.DB) error {
		updatedResources := []*model_legacy.Resource{}
		resource, err := r.FindByIDWithTx(ctx, tx, id)
		if err != nil {
			return err
		}

		if err := tx.Create(copyHistory(m, id, model_legacy.OperationTypeUpdate)).Error; err != nil {
			return err
		}

		m.ID = id
		m.CreatedAt = resource.CreatedAt
		m.InventoryId = resource.InventoryId
		if err := tx.Save(m).Error; err != nil {
			return err
		}

		// Handle workspace updates for other resources with the same inventory ID
		updatedResources, err = r.handleWorkspaceUpdates(tx, m, updatedResources)
		if err != nil {
			return err
		}

		// Publish outbox events for primary resource
		err = handleOutboxEvents(tx, *m, namespace, model_legacy.OperationTypeUpdated, txid)
		if err != nil {
			return err
		}

		// Publish outbox event for the primary resource and other resources with the same inventory ID
		for _, updatedResource := range updatedResources {
			err = handleOutboxEvents(tx, *updatedResource, namespace, model_legacy.OperationTypeUpdated, "")
			if err != nil {
				return err
			}
		}
		result = m
		return nil
	})

	if err != nil {
		return nil, err
	}
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model_legacy.OperationTypeUpdated), nil)
	return result, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model_legacy.Resource, error) {
	db := r.DB.Session(&gorm.Session{})
	var result *model_legacy.Resource
	err := r.TransactionManager.HandleSerializableTransaction("LegacyDeleteResource", db, func(tx *gorm.DB) error {
		resource, err := r.FindByIDWithTx(ctx, tx, id)
		if err != nil {
			return err
		}

		if err := tx.Create(copyHistory(resource, resource.ID, model_legacy.OperationTypeDelete)).Error; err != nil {
			return err
		}

		// Delete relationships - We don't yet care about keeping history of deleted relationships of a deleted resource.
		if err := tx.Where("subject_id = ? or object_id = ?", id, id).Delete(&model_legacy.Relationship{}).Error; err != nil {
			return err
		}

		if err := tx.Delete(resource).Error; err != nil {
			return err
		}

		if resource.InventoryId != nil {
			// Delete Inventory Resource if no other resources are referencing it
			var count int64
			if err := tx.Model(&model_legacy.Resource{}).Where("inventory_id = ?", *resource.InventoryId).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				if err := tx.Delete(&model_legacy.InventoryResource{}, *resource.InventoryId).Error; err != nil {
					return err
				}
			}
		}

		err = handleOutboxEvents(tx, *resource, namespace, model_legacy.OperationTypeDeleted, "")
		if err != nil {
			return err
		}

		result = resource
		return nil
	})

	if err != nil {
		return nil, err
	}
	metricscollector.Incr(r.MetricsCollector.OutboxEventWrites, string(model_legacy.OperationTypeDeleted), nil)
	return result, nil
}

func (r *Repo) FindByIDWithTx(ctx context.Context, tx *gorm.DB, id uuid.UUID) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := tx.First(&resource, id).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := r.DB.Session(&gorm.Session{}).First(&resource, id).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model_legacy.Resource, error) {
	session := r.DB.Session(&gorm.Session{})
	data := []*model_legacy.Resource{}

	log.Infof("FindByWorkspaceId: %s", workspace_id)
	if err := session.Where("workspace_id = ?", workspace_id).Find(&data).Error; err != nil {
		return nil, err
	}

	log.Infof("FindByWorkspaceId: data %+v", data)
	return data, nil
}

// Deprecated: Prefer FindByReporterData instead
func (r *Repo) FindByReporterResourceId(ctx context.Context, id model_legacy.ReporterResourceId) (*model_legacy.Resource, error) {
	session := r.DB.Session(&gorm.Session{})

	resourceId, err := data.GetLastResourceId(session, id)
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, resourceId)
}

func (r *Repo) FindByReporterResourceIdv1beta2(ctx context.Context, id model_legacy.ReporterResourceUniqueIndex) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model_legacy.ReporterResourceUniqueIndex{
		ReporterInstanceId: id.ReporterInstanceId,
		ReporterResourceId: id.ReporterResourceId,
		ResourceType:       id.ResourceType,
		ReporterType:       id.ReporterType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model_legacy.Resource{
		InventoryId:  inventoryId,
		ResourceType: resourceType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterInstanceId string, reporterType string) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model_legacy.Resource{
		InventoryId:        inventoryId,
		ReporterInstanceId: reporterInstanceId,
		ReporterType:       reporterType,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByReporterData(ctx context.Context, reporterId string, reporterResourceId string) (*model_legacy.Resource, error) {
	resource := model_legacy.Resource{}
	if err := r.DB.Session(&gorm.Session{}).Where(&model_legacy.Resource{
		ReporterId:         reporterId,
		ReporterResourceId: reporterResourceId,
	}).First(&resource).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) ListAll(context.Context) ([]*model_legacy.Resource, error) {
	var results []*model_legacy.Resource
	if err := r.DB.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}

func (r *Repo) handleWorkspaceUpdates(tx *gorm.DB, m *model_legacy.Resource, updatedResources []*model_legacy.Resource) ([]*model_legacy.Resource, error) {
	if m.InventoryId != nil {
		var inventoryResource model_legacy.InventoryResource
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
			var resources []model_legacy.Resource
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

func handleOutboxEvents(tx *gorm.DB, resource model_legacy.Resource, namespace string, operationType model_legacy.EventOperationType, txid string) error {
	resourceMessage, tupleMessage, err := model_legacy.NewOutboxEventsFromResource(resource, namespace, operationType, txid)
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
