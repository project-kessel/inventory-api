package resources

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	"gorm.io/gorm"
)

type Repo struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
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

func (r *Repo) Create(ctx context.Context, m *model.Resource) (*model.Resource, []*model.Resource, error) {
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
			return nil, nil, fmt.Errorf("creating inventory resource: %w", err)
		}
		m.InventoryId = &inventoryResource.ID
	}

	if err := tx.Create(m).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	if err := tx.Create(copyHistory(m, m.ID, model.OperationTypeCreate)).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	// Handle workspace updates for other resources with the same inventory ID
	updatedResources, err := r.handleWorkspaceUpdates(tx, m, updatedResources)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	// Deprecated
	// TODO: Remove this when all resources are created with inventory ID
	if err := tx.Create(&model.LocalInventoryToResource{
		ResourceId:         m.ID,
		ReporterResourceId: model.ReporterResourceIdFromResource(m),
	}).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	tx.Commit()
	return m, updatedResources, nil
}

func (r *Repo) Update(ctx context.Context, m *model.Resource, id uuid.UUID) (*model.Resource, []*model.Resource, error) {
	db := r.DB.Session(&gorm.Session{})
	updatedResources := []*model.Resource{}

	resource, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	tx := db.Begin()
	if err := tx.Create(copyHistory(m, id, model.OperationTypeUpdate)).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	m.ID = id
	m.CreatedAt = resource.CreatedAt
	if err := tx.Save(m).Error; err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	updatedResources = append(updatedResources, m)

	// Handle workspace updates for other resources with the same inventory ID
	updatedResources, err = r.handleWorkspaceUpdates(tx, m, updatedResources)
	if err != nil {
		tx.Rollback()
		return nil, nil, err
	}

	tx.Commit()
	return m, updatedResources, nil
}

func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
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

	tx.Commit()
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
	if err := session.Where("workspace_id = ?", workspace_id).Find(&data).Error; err == nil {
		log.Infof("FindByWorkspaceId: data %+v", data)
		return data, nil
	} else {
		return nil, err
	}
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
