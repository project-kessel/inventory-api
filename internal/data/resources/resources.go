package resources

import (
	"context"
	"fmt"

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

func (r *Repo) Create(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	var inventoryResource model.InventoryResource
	db := r.DB.Session(&gorm.Session{})
	tx := db.Begin()

	// If reporter includes inventory ID, check if it exists
	if m.InventoryId != uuid.Nil {
		if err := tx.First(&inventoryResource, m.InventoryId).Error; err != nil {
			// Bad Inventory ID
			tx.Rollback()
			return nil, fmt.Errorf("fetching inventory resource: %w", err)
		}
		m.InventoryId = inventoryResource.ID
	} else {
		// Create a new inventory resource
		if err := tx.Create(&inventoryResource).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("creating inventory resource: %w", err)
		}
		m.InventoryId = inventoryResource.ID
	}

	if err := tx.Create(m).Error; err != nil {
		tx.Rollback()
		return nil, err
	}

	if err := tx.Create(copyHistory(m, m.ID, model.OperationTypeCreate)).Error; err != nil {
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

	tx.Commit()
	return m, nil
}

func (r *Repo) Update(ctx context.Context, m *model.Resource, id uuid.UUID) (*model.Resource, error) {
	session := r.DB.Session(&gorm.Session{})

	_, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(m, id, model.OperationTypeUpdate)).Error; err != nil {
		return nil, err
	}

	m.ID = id
	if err := session.Save(m).Error; err != nil {
		return nil, err
	}

	return m, nil
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

	// Delete Inventory Resource if no other resources are referencing it
	var count int64
	if err := tx.Model(&model.Resource{}).Where("inventory_id = ?", resource.InventoryId).Count(&count).Error; err != nil {
		tx.Rollback()
		return nil, err
	}
	if count == 0 {
		if err := tx.Delete(&model.InventoryResource{}, resource.InventoryId).Error; err != nil {
			tx.Rollback()
			return nil, err
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
