package resources

import (
	"context"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type Repo struct {
	DB      *gorm.DB
	Eventer eventingapi.Manager
}

func New(db *gorm.DB, eventer eventingapi.Manager) *Repo {
	return &Repo{
		DB:      db,
		Eventer: eventer,
	}
}

func (r *Repo) Save(ctx context.Context, model *model.Relationship) (*model.Relationship, error) {
	if err := r.DB.Session(&gorm.Session{FullSaveAssociations: true}).Create(model).Error; err != nil {
		return nil, err
	}

	if r.Eventer != nil {
		err := data.DefaultRelationshipSendEvent(ctx, model, r.Eventer, *model.CreatedAt, eventingapi.OperationTypeCreated)

		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Update(ctx context.Context, model *model.Relationship, id uint64) (*model.Relationship, error) {
	// TODO: update the model in inventory

	if r.Eventer != nil {
		err := data.DefaultRelationshipSendEvent(ctx, model, r.Eventer, *model.UpdatedAt, eventingapi.OperationTypeUpdated)

		if err != nil {
			return nil, err
		}
	}

	return model, nil
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Delete(ctx context.Context, id uint64) error {
	// TODO: Retrieve data from inventory so we have something to publish
	model := &model.Relationship{
		ID:               0,
		RelationshipData: nil,
		RelationshipType: "",
		SubjectId:        0,
		ObjectId:         0,
		Reporter:         model.RelationshipReporter{},
	}

	// TODO: delete the model from inventory

	if r.Eventer != nil {
		err := data.DefaultRelationshipSendEvent(ctx, model, r.Eventer, time.Now(), eventingapi.OperationTypeDeleted)

		if err != nil {
			return err
		}
	}

	// TODO: delete the workspace tuple

	return nil
}

func (r *Repo) FindByID(context.Context, uint64) (*model.Relationship, error) {
	return nil, nil
}

func (r *Repo) ListAll(context.Context) ([]*model.Relationship, error) {
	// var model biz.Resource
	// var count int64
	// if err := r.Db.Model(&model).Count(&count).Error; err != nil {
	// 	return nil, err
	// }

	var results []*model.Relationship
	if err := r.DB.Preload(clause.Associations).Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
