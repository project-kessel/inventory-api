package resources

import (
	"context"
	"errors"
	"time"

	"github.com/project-kessel/inventory-api/internal/middleware"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
)

type ResourceRepository interface {
	Save(ctx context.Context, resource *model_legacy.Relationship) (*model_legacy.Relationship, error)
	Update(context.Context, *model_legacy.Relationship, uuid.UUID) (*model_legacy.Relationship, error)
	Delete(context.Context, uuid.UUID) (*model_legacy.Relationship, error)
	FindByID(context.Context, uuid.UUID) (*model_legacy.Relationship, error)
	FindRelationship(ctx context.Context, subjectId, objectId uuid.UUID, relationshipType string) (*model_legacy.Relationship, error)
	FindResourceIdByReporterResourceId(ctx context.Context, id model_legacy.ReporterResourceId) (uuid.UUID, error)
	ListAll(context.Context) ([]*model_legacy.Relationship, error)
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

func (uc *Usecase) Create(ctx context.Context, m *model_legacy.Relationship) (*model_legacy.Relationship, error) {
	var ret *model_legacy.Relationship
	relationshipId := model_legacy.ReporterRelationshipIdFromRelationship(m)

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

	ret, err = uc.repository.Save(ctx, m)
	if err != nil {
		return nil, err
	}

	if uc.eventer != nil {
		err := DefaultRelationshipSendEvent(ctx, m, uc.eventer, *m.CreatedAt, biz.OperationTypeCreated)

		if err != nil {
			return nil, err
		}
	}

	uc.log.WithContext(ctx).Infof("Created Relationship: %v(%v)", m.ID, m.RelationshipType)
	return ret, nil
}

func (uc *Usecase) Update(ctx context.Context, m *model_legacy.Relationship, id model_legacy.ReporterRelationshipId) (*model_legacy.Relationship, error) {
	var ret *model_legacy.Relationship
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

	ret, err = uc.repository.Update(ctx, m, existingResource.ID)
	if err != nil {
		return nil, err
	}

	if uc.eventer != nil {
		err := DefaultRelationshipSendEvent(ctx, m, uc.eventer, *m.UpdatedAt, biz.OperationTypeUpdated)

		if err != nil {
			return nil, err
		}
	}

	uc.log.WithContext(ctx).Infof("Updated Relationship: %v(%v)", m.ID, m.RelationshipType)
	return ret, nil
}

func (uc *Usecase) Delete(ctx context.Context, id model_legacy.ReporterRelationshipId) error {
	var m *model_legacy.Relationship

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

	m, err = uc.repository.Delete(ctx, existingResource.ID)
	if err != nil {
		return err
	}

	if uc.eventer != nil {
		err := DefaultRelationshipSendEvent(ctx, m, uc.eventer, time.Now(), biz.OperationTypeDeleted)

		if err != nil {
			return err
		}
	}

	uc.log.WithContext(ctx).Infof("Deleted Relationship: %v(%v)", m.ID, m.RelationshipType)
	return nil
}

// Moved here from common.go since it's not used outside this file, so keeping it here avoids maintaining an unnecessary common.go.

func DefaultRelationshipSendEvent(ctx context.Context, m *model_legacy.Relationship, eventer eventingapi.Manager, reportedTime time.Time, operationType biz.EventOperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, m.RelationshipType, m.ID)
	evt, err := eventingapi.NewRelationshipEvent(operationType, m, reportedTime)
	if err != nil {
		return err
	}

	err = producer.Produce(ctx, evt)
	if err != nil {
		return err
	}

	return nil
}
