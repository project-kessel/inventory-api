package resources

import (
	"context"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repo struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

func (r *Repo) Save(ctx context.Context, model *model.Resource) (*model.Resource, error) {
	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	return model, nil
}

func (r *Repo) Update(ctx context.Context, model *model.Resource, id uint64) (*model.Resource, error) {
	// TODO: update the model in inventory
	return model, nil
}

func (r *Repo) Delete(ctx context.Context, id uint64) error {
	return nil
}

func (r *Repo) FindByID(context.Context, uint64) (*model.Resource, error) {
	return nil, nil
}

func (r *Repo) ListAll(context.Context) ([]*model.Resource, error) {
	// var model biz.Resource
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*model.Resource
	if err := r.DB.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
