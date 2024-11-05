package resources

import (
	"context"
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
		Reporter:      m.Reporter,
		ConsoleHref:   m.ConsoleHref,
		ApiHref:       m.ApiHref,
		Labels:        m.Labels,
		ResourceId:    id,
		OperationType: operationType,
	}
}

func (r *Repo) Save(ctx context.Context, m *model.Resource) (*model.Resource, error) {
	session := r.DB.Session(&gorm.Session{})

	if err := session.Create(m).Error; err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(m, m.ID, model.OperationTypeCreate)).Error; err != nil {
		return nil, err
	}

	if err := session.Create(&model.LocalInventoryToResource{
		ResourceId:         m.ID,
		ReporterResourceId: model.ReporterResourceIdFromResource(m),
	}).Error; err != nil {
		return nil, err
	}

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
	session := r.DB.Session(&gorm.Session{})

	resource, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(resource, resource.ID, model.OperationTypeDelete)).Error; err != nil {
		return nil, err
	}

	// Delete relationships - We don't yet care about keeping history of deleted relationships of a deleted resource.
	if err := session.Where("subject_id = ? or object_id = ?", id, id).Delete(&model.Relationship{}).Error; err != nil {
		return nil, err
	}

	if err := session.Delete(resource).Error; err != nil {
		return nil, err
	}

	return resource, nil
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	resource := model.Resource{}
	if err := r.DB.Session(&gorm.Session{}).First(&resource, id).Error; err != nil {
		return nil, err
	}

	return &resource, nil
}

func (r *Repo) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	session := r.DB.Session(&gorm.Session{})

	resourceId, err := data.GetLastResourceId(session, id)
	if err != nil {
		return nil, err
	}

	return r.FindByID(ctx, resourceId)
}

func (r *Repo) ListAll(context.Context) ([]*model.Resource, error) {
	var results []*model.Resource
	if err := r.DB.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
