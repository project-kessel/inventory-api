package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

type MockedRelationshipRepository struct {
	mock.Mock
}

func (r *MockedRelationshipRepository) Save(ctx context.Context, resource *model_legacy.Relationship) (*model_legacy.Relationship, error) {
	args := r.Called(ctx, resource)
	return args.Get(0).(*model_legacy.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) Update(ctx context.Context, resource *model_legacy.Relationship, id uuid.UUID) (*model_legacy.Relationship, error) {
	args := r.Called(ctx, resource, id)
	return args.Get(0).(*model_legacy.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) Delete(ctx context.Context, id uuid.UUID) (*model_legacy.Relationship, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindByID(ctx context.Context, id uuid.UUID) (*model_legacy.Relationship, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model_legacy.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindRelationship(ctx context.Context, subjectId, objectId uuid.UUID, relationshipType string) (*model_legacy.Relationship, error) {
	args := r.Called(ctx, subjectId, objectId, relationshipType)
	return args.Get(0).(*model_legacy.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindResourceIdByReporterResourceId(ctx context.Context, id model_legacy.ReporterResourceId) (uuid.UUID, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

func (r *MockedRelationshipRepository) ListAll(ctx context.Context) ([]*model_legacy.Relationship, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model_legacy.Relationship), args.Error(1)
}

var (
	orgId                  = "my-org"
	reporterId             = "my-reporter-id"
	reporterType           = "my-reporter-type"
	subjectLocalResourceId = "software-01"
	subjectResourceType    = "software"
	objectLocalResourceId  = "heart-hemorrhage"
	objectResourceType     = "bug"
)

func relationship1(subjectId, objectId uuid.UUID) *model_legacy.Relationship {
	return &model_legacy.Relationship{
		ID:               uuid.UUID{},
		OrgId:            orgId,
		RelationshipData: nil,
		RelationshipType: "software_has-a-bug_bug",
		SubjectId:        subjectId,
		ObjectId:         objectId,
		Reporter: model_legacy.RelationshipReporter{
			Reporter: model_legacy.Reporter{
				ReporterId:      reporterId,
				ReporterType:    reporterType,
				ReporterVersion: "3.14.159",
			},
			SubjectLocalResourceId: subjectLocalResourceId,
			SubjectResourceType:    "software",
			ObjectLocalResourceId:  objectLocalResourceId,
			ObjectResourceType:     "bug",
		},
		CreatedAt: nil,
		UpdatedAt: nil,
	}
}

func TestCreateRelationshipAlreadyExists(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	repo := &MockedRelationshipRepository{}

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(sid, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(oid, nil).Once()
	repo.On("FindRelationship", mock.Anything, sid, oid, mock.Anything).Return(&model_legacy.Relationship{}, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err = useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrRelationshipExists)
	repo.AssertExpectations(t)
}

func TestCreateSubjectNotFound(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	repo := &MockedRelationshipRepository{}

	// Subject not found
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model_legacy.ReporterResourceId{
		LocalResourceId: subjectLocalResourceId,
		ResourceType:    subjectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return(uuid.Nil, gorm.ErrRecordNotFound).Once()

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err = useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrSubjectNotFound)
	repo.AssertExpectations(t)
}

func TestCreateObjectNotFound(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	repo := &MockedRelationshipRepository{}

	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model_legacy.ReporterResourceId{
		LocalResourceId: subjectLocalResourceId,
		ResourceType:    subjectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return(sid, nil)
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model_legacy.ReporterResourceId{
		LocalResourceId: objectLocalResourceId,
		ResourceType:    objectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return(uuid.Nil, gorm.ErrRecordNotFound)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err = useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrObjectNotFound)
	repo.AssertExpectations(t)
}

func TestCreateNewRelationship(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	rid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	repo := &MockedRelationshipRepository{}
	returnedRelationship := model_legacy.Relationship{
		ID: rid,
	}

	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(sid, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(oid, nil).Once()
	repo.On("FindRelationship", mock.Anything, sid, oid, mock.Anything).Return((*model_legacy.Relationship)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	rCreated, err := useCase.Create(ctx, r)
	assert.Nil(t, err)
	assert.Equal(t, sid, r.SubjectId)
	assert.Equal(t, oid, r.ObjectId)
	assert.Equal(t, &returnedRelationship, rCreated)
	repo.AssertExpectations(t)
}

func TestUpdateNewRelationshipCreatesIt(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	rid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	repo := &MockedRelationshipRepository{}
	returnedRelationship := model_legacy.Relationship{
		ID: rid,
	}

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(sid, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(oid, nil).Once()
	// Create calls these again
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(sid, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(oid, nil).Once()

	repo.On("FindRelationship", mock.Anything, sid, oid, mock.Anything).Return((*model_legacy.Relationship)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	rCreated, err := useCase.Update(ctx, r, model_legacy.ReporterRelationshipId{})
	assert.Nil(t, err)
	assert.Equal(t, sid, r.SubjectId)
	assert.Equal(t, oid, r.ObjectId)
	assert.Equal(t, &returnedRelationship, rCreated)
	repo.AssertExpectations(t)
}

func TestUpdateExistingRelationship(t *testing.T) {
	sid, err := uuid.NewV7()
	assert.Nil(t, err)

	oid, err := uuid.NewV7()
	assert.Nil(t, err)

	rid, err := uuid.NewV7()
	assert.Nil(t, err)

	r := relationship1(sid, oid)
	r.ID = rid

	repo := &MockedRelationshipRepository{}
	returnRelationship := model_legacy.Relationship{
		ID:    r.ID,
		OrgId: "my-new-org",
	}

	// Resource exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(sid, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(oid, nil).Once()
	repo.On("FindRelationship", mock.Anything, sid, oid, mock.Anything).Return(r, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	rUpdated, err := useCase.Update(ctx, r, model_legacy.ReporterRelationshipId{})
	assert.Nil(t, err)
	assert.Equal(t, sid, r.SubjectId)
	assert.Equal(t, oid, r.ObjectId)
	assert.Equal(t, rid, r.ID)
	assert.Equal(t, &returnRelationship, rUpdated)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentRelationship(t *testing.T) {
	repo := &MockedRelationshipRepository{}

	// Resource does not exist
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(uuid.Nil, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(uuid.Nil, nil).Once()
	repo.On("FindRelationship", mock.Anything, uuid.Nil, uuid.Nil, mock.Anything).Return((*model_legacy.Relationship)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model_legacy.ReporterRelationshipId{})
	assert.Error(t, err, "resource not found")
	repo.AssertExpectations(t)
}

func TestDeleteRelationship(t *testing.T) {
	rid, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedRelationshipRepository{}
	ctx := context.TODO()

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(uuid.Nil, nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return(uuid.Nil, nil).Once()
	repo.On("FindRelationship", mock.Anything, uuid.Nil, uuid.Nil, mock.Anything).Return(&model_legacy.Relationship{
		ID: rid,
	}, nil)
	repo.On("Delete", mock.Anything, rid).Return(&model_legacy.Relationship{}, nil)

	useCase := New(repo, nil, log.DefaultLogger)

	err = useCase.Delete(ctx, model_legacy.ReporterRelationshipId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}
