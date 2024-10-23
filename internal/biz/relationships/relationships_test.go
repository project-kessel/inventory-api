package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
	"testing"
)

type MockedRelationshipRepository struct {
	mock.Mock
}

func (r *MockedRelationshipRepository) Save(ctx context.Context, resource *model.Relationship) (*model.Relationship, error) {
	args := r.Called(ctx, resource)
	return args.Get(0).(*model.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) Update(ctx context.Context, resource *model.Relationship, id uint64) (*model.Relationship, error) {
	args := r.Called(ctx, resource, id)
	return args.Get(0).(*model.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) Delete(ctx context.Context, id uint64) (*model.Relationship, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindByID(ctx context.Context, id uint64) (*model.Relationship, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindRelationship(ctx context.Context, subjectId, objectId uint64, relationshipType string) (*model.Relationship, error) {
	args := r.Called(ctx, subjectId, objectId, relationshipType)
	return args.Get(0).(*model.Relationship), args.Error(1)
}

func (r *MockedRelationshipRepository) FindResourceIdByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (uint64, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(uint64), args.Error(1)
}

func (r *MockedRelationshipRepository) ListAll(ctx context.Context) ([]*model.Relationship, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Relationship), args.Error(1)
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

func relationship1(subjectId, objectId uint64) *model.Relationship {
	return &model.Relationship{
		ID:               0,
		OrgId:            orgId,
		RelationshipData: nil,
		RelationshipType: "software_has-a-bug_bug",
		SubjectId:        subjectId,
		ObjectId:         objectId,
		Reporter: model.RelationshipReporter{
			Reporter: model.Reporter{
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

func TestCreateResourceAlreadyExists(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return(&model.Relationship{}, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrRelationshipExists)
	repo.AssertExpectations(t)
}

func TestCreateSubjectNotFound(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}

	// Subject not found
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model.ReporterResourceId{
		LocalResourceId: subjectLocalResourceId,
		ResourceType:    subjectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return((uint64)(0), gorm.ErrRecordNotFound).Once()

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrSubjectNotFound)
	repo.AssertExpectations(t)
}

func TestCreateObjectNotFound(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}

	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model.ReporterResourceId{
		LocalResourceId: subjectLocalResourceId,
		ResourceType:    subjectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return((uint64)(1), nil)
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, model.ReporterResourceId{
		LocalResourceId: objectLocalResourceId,
		ResourceType:    objectResourceType,
		ReporterId:      reporterId,
		ReporterType:    reporterType,
	}).Return((uint64)(0), gorm.ErrRecordNotFound)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, r)
	assert.ErrorIs(t, err, ErrObjectNotFound)
	repo.AssertExpectations(t)
}

func TestCreateNewResource(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}
	returnedRelationship := model.Relationship{
		ID: 10,
	}

	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return((*model.Relationship)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, r)
	assert.Nil(t, err)
	assert.Equal(t, &returnedRelationship, r)
	repo.AssertExpectations(t)
}

func TestUpdateNewResourceCreatesIt(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}
	returnedRelationship := model.Relationship{
		ID: 10,
	}

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	// Create calls these again
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()

	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return((*model.Relationship)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, r, model.ReporterRelationshipId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedRelationship, r)
	repo.AssertExpectations(t)
}

func TestUpdateExistingResource(t *testing.T) {
	r := relationship1(0, 0)
	repo := &MockedRelationshipRepository{}
	returnRelationship := model.Relationship{
		ID: 10,
	}

	// Resource does not exist
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return(&model.Relationship{
		ID: 33,
	}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnRelationship, nil)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, r, model.ReporterRelationshipId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnRelationship, r)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentResource(t *testing.T) {
	repo := &MockedRelationshipRepository{}

	// Resource does not exist
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return((*model.Relationship)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, nil, log.DefaultLogger)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterRelationshipId{})
	assert.Error(t, err, "resource not found")
	repo.AssertExpectations(t)
}

func TestDeleteResource(t *testing.T) {
	repo := &MockedRelationshipRepository{}
	ctx := context.TODO()

	// Resource already exists
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(1), nil).Once()
	repo.On("FindResourceIdByReporterResourceId", mock.Anything, mock.Anything).Return((uint64)(2), nil).Once()
	repo.On("FindRelationship", mock.Anything, (uint64)(1), (uint64)(2), mock.Anything).Return(&model.Relationship{
		ID: 33,
	}, nil)
	repo.On("Delete", mock.Anything, (uint64)(33)).Return(&model.Relationship{}, nil)

	useCase := New(repo, nil, log.DefaultLogger)

	err := useCase.Delete(ctx, model.ReporterRelationshipId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}
