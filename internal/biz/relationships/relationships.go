package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

type ResourceRepository interface {
	Save(ctx context.Context, resource *model.Relationship) (*model.Relationship, error)
	Update(context.Context, *model.Relationship, uint64) (*model.Relationship, error)
	Delete(context.Context, uint64) error
	FindByID(context.Context, uint64) (*model.Relationship, error)
	ListAll(context.Context) ([]*model.Relationship, error)
}

type Usecase struct {
	repository ResourceRepository
	log        *log.Helper
}

func New(repository ResourceRepository, logger log.Logger) *Usecase {
	return &Usecase{
		repository: repository,
		log:        log.NewHelper(logger),
	}
}

func (uc *Usecase) Create(ctx context.Context, m *model.Relationship) (*model.Relationship, error) {
	if ret, err := uc.repository.Save(ctx, m); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Created Relationship: %v(%v)", m.ID, m.RelationshipType)
		return ret, nil
	}
}

// Update updates a model in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (uc *Usecase) Update(ctx context.Context, m *model.Relationship, id string) (*model.Relationship, error) {
	if ret, err := uc.repository.Update(ctx, m, 0); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Updated Relationship: %v(%v)", m.ID, m.RelationshipType)
		return ret, nil
	}
}

// Delete deletes a model from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (uc *Usecase) Delete(ctx context.Context, id string) error {
	if err := uc.repository.Delete(ctx, 0); err != nil {
		return err
	} else {
		// TODO: Retrieve data from inventory so we have something to publish
		m := &model.Relationship{
			ID:               0,
			OrgId:            "",
			RelationshipData: nil,
			RelationshipType: "",
			SubjectId:        0,
			ObjectId:         0,
			Reporter:         model.RelationshipReporter{},
			CreatedAt:        nil,
			UpdatedAt:        nil,
		}

		// TODO: delete the model from inventory
		uc.log.WithContext(ctx).Infof("Deleted Relationship: %v(%v)", m.ID, m.RelationshipType)
		return nil
	}
}
