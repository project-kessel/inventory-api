package resources

import (
	"context"
	"errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"gorm.io/gorm"
	"time"
)

type ResourceRepository interface {
	Save(ctx context.Context, resource *model.Relationship) (*model.Relationship, error)
	Update(context.Context, *model.Relationship, uuid.UUID) (*model.Relationship, error)
	Delete(context.Context, uuid.UUID) (*model.Relationship, error)
	FindByID(context.Context, uuid.UUID) (*model.Relationship, error)
	FindRelationship(ctx context.Context, subjectId, objectId uuid.UUID, relationshipType string) (*model.Relationship, error)
	FindResourceIdByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (uuid.UUID, error)
	ListAll(context.Context) ([]*model.Relationship, error)
}

type Usecase struct {
	repository ResourceRepository
	eventer    eventingapi.Manager
	log        *log.Helper
}

var (
	ErrSubjectNotFound      = errors.New("subject not found")
	ErrObjectNotFound       = errors.New("object not found")
	ErrRelationshipExists   = errors.New("relationship already exists")
	ErrRelationshipNotFound = errors.New("relationship not found")
)

func New(repository ResourceRepository, eventer eventingapi.Manager, logger log.Logger) *Usecase {
	return &Usecase{
		repository: repository,
		eventer:    eventer,
		log:        log.NewHelper(logger),
	}
}

func (uc *Usecase) Create(ctx context.Context, m *model.Relationship) (*model.Relationship, error) {
	relationshipId := model.ReporterRelationshipIdFromRelationship(m)

	subjectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, relationshipId.SubjectId)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	objectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, relationshipId.ObjectId)
	if err != nil {
		return nil, ErrObjectNotFound
	}

	// check if the relationship already exists
	_, err = uc.repository.FindRelationship(ctx, subjectId, objectId, relationshipId.RelationshipType)

	if err == nil || !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrRelationshipExists
	}

	m.SubjectId = subjectId
	m.ObjectId = objectId

	if ret, err := uc.repository.Save(ctx, m); err != nil {
		return nil, err
	} else {
		if uc.eventer != nil {
			err := biz.DefaultRelationshipSendEvent(ctx, m, uc.eventer, *m.CreatedAt, eventingapi.OperationTypeCreated)

			if err != nil {
				return nil, err
			}
		}

		uc.log.WithContext(ctx).Infof("Created Relationship: %v(%v)", m.ID, m.RelationshipType)
		return ret, nil
	}
}

func (uc *Usecase) Update(ctx context.Context, m *model.Relationship, id model.ReporterRelationshipId) (*model.Relationship, error) {
	subjectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, id.SubjectId)
	if err != nil {
		return nil, ErrSubjectNotFound
	}

	objectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, id.ObjectId)
	if err != nil {
		return nil, ErrObjectNotFound
	}

	// check if the relationship already exists
	existingResource, err := uc.repository.FindRelationship(ctx, subjectId, objectId, id.RelationshipType)

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return uc.Create(ctx, m)
	}

	m.SubjectId = subjectId
	m.ObjectId = objectId

	if ret, err := uc.repository.Update(ctx, m, existingResource.ID); err != nil {
		return nil, err
	} else {
		if uc.eventer != nil {
			err := biz.DefaultRelationshipSendEvent(ctx, m, uc.eventer, *m.UpdatedAt, eventingapi.OperationTypeUpdated)

			if err != nil {
				return nil, err
			}
		}

		uc.log.WithContext(ctx).Infof("Updated Relationship: %v(%v)", m.ID, m.RelationshipType)
		return ret, nil
	}
}

func (uc *Usecase) Delete(ctx context.Context, id model.ReporterRelationshipId) error {
	subjectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, id.SubjectId)
	if err != nil {
		return ErrSubjectNotFound
	}

	objectId, err := uc.repository.FindResourceIdByReporterResourceId(ctx, id.ObjectId)
	if err != nil {
		return ErrObjectNotFound
	}

	// check if the relationship already exists
	existingResource, err := uc.repository.FindRelationship(ctx, subjectId, objectId, id.RelationshipType)

	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrRelationshipNotFound
	}

	if m, err := uc.repository.Delete(ctx, existingResource.ID); err != nil {
		return err
	} else {
		if uc.eventer != nil {
			err := biz.DefaultRelationshipSendEvent(ctx, m, uc.eventer, time.Now(), eventingapi.OperationTypeDeleted)

			if err != nil {
				return err
			}
		}

		uc.log.WithContext(ctx).Infof("Deleted Relationship: %v(%v)", m.ID, m.RelationshipType)
		return nil
	}
}
