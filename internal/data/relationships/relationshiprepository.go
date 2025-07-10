package resources

import (
	"context"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/data"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
)

type Repo struct {
	DB      *gorm.DB
	Eventer eventingapi.Manager
}

func New(db *gorm.DB) *Repo {
	return &Repo{
		DB: db,
	}
}

func copyHistory(m *model_legacy.Relationship, id uuid.UUID, operationType model_legacy.OperationType) *model_legacy.RelationshipHistory {
	return &model_legacy.RelationshipHistory{
		OrgId:            m.OrgId,
		RelationshipData: m.RelationshipData,
		RelationshipType: m.RelationshipType,
		SubjectId:        m.SubjectId,
		ObjectId:         m.ObjectId,
		Reporter:         m.Reporter,
		RelationshipId:   id,
		OperationType:    operationType,
	}
}

func (r *Repo) Save(ctx context.Context, m *model_legacy.Relationship) (*model_legacy.Relationship, error) {
	session := r.DB.Session(&gorm.Session{})

	if err := session.Create(m).Error; err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(m, m.ID, model_legacy.OperationTypeCreate)).Error; err != nil {
		return nil, err
	}

	return m, nil
}

// Update updates a model_legacy in the database, updates related tuples in the relations-api, and issues an update event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Update(ctx context.Context, m *model_legacy.Relationship, id uuid.UUID) (*model_legacy.Relationship, error) {
	session := r.DB.Session(&gorm.Session{})

	_, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(m, id, model_legacy.OperationTypeUpdate)).Error; err != nil {
		return nil, err
	}

	m.ID = id
	if err := session.Save(m).Error; err != nil {
		return nil, err
	}

	return m, nil
}

// Delete deletes a model_legacy from the database, removes related tuples from the relations-api, and issues a delete event.
// The `id` is possibly of the form <reporter_type:local_resource_id>.
func (r *Repo) Delete(ctx context.Context, id uuid.UUID) (*model_legacy.Relationship, error) {
	session := r.DB.Session(&gorm.Session{})

	relationship, err := r.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if err := session.Create(copyHistory(relationship, relationship.ID, model_legacy.OperationTypeDelete)).Error; err != nil {
		return nil, err
	}

	if err := session.Delete(relationship).Error; err != nil {
		return nil, err
	}

	return relationship, nil
}

func (r *Repo) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.Relationship, error) {
	relationship := model_legacy.Relationship{}
	if err := r.DB.Session(&gorm.Session{}).First(&relationship, id).Error; err != nil {
		return nil, err
	}

	return &relationship, nil
}

func (r *Repo) FindResourceIdByReporterResourceId(ctx context.Context, id model_legacy.ReporterResourceId) (uuid.UUID, error) {
	return data.GetLastResourceId(r.DB.Session(&gorm.Session{}), id)
}

func (r *Repo) FindRelationship(ctx context.Context, subjectId, objectId uuid.UUID, relationshipType string) (*model_legacy.Relationship, error) {
	session := r.DB.Session(&gorm.Session{})
	relation := model_legacy.Relationship{}

	err := session.First(
		&relation,
		"subject_id = (?) AND object_id = (?) AND relationship_type = ?",
		subjectId,
		objectId,
		relationshipType,
	).Error

	if err != nil {
		return nil, err
	}

	return &relation, nil
}

func (r *Repo) ListAll(context.Context) ([]*model_legacy.Relationship, error) {
	var results []*model_legacy.Relationship
	if err := r.DB.Find(&results).Error; err != nil {
		return nil, err
	}

	return results, nil
}
