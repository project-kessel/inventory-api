package resources

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

type MockedReporterResourceRepository struct {
	mock.Mock
}
type MockedInventoryResourceRepository struct {
	mock.Mock
}

func (r *MockedReporterResourceRepository) Create(ctx context.Context, resource *model.Resource, namespace string) (*model.Resource, error) {
	args := r.Called(ctx, resource)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Update(ctx context.Context, resource *model.Resource, id uuid.UUID, namespace string) (*model.Resource, error) {
	args := r.Called(ctx, resource, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterData(ctx context.Context, reporterId string, resourceId string) (*model.Resource, error) {
	args := r.Called(ctx, reporterId, resourceId)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) ListAll(ctx context.Context) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (r *MockedInventoryResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.InventoryResource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.InventoryResource), args.Error(1)
}

func resource1() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org",
		ResourceData: map[string]any{
			"foo": "bar",
		},
		ResourceType: "my-resource",
		WorkspaceId:  "my-workspace",
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "reporter_id",
				ReporterType:    "reporter_type",
				ReporterVersion: "1.0.2",
			},
			LocalResourceId: "foo-resource",
		},
		ConsoleHref: "/etc/console",
		ApiHref:     "/etc/api",
		Labels: model.Labels{
			{
				Key:   "label-1",
				Value: "value-1",
			},
			{
				Key:   "label-1",
				Value: "value-2",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

func TestCreateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility, record was not found via new method
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExists(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExistsBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateNewResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestUpdateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestUpdateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestUpdateNewResourceCreatesIt(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource doesn't exist
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestUpdateExistingResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
}
func TestUpdateExistingResourceBackwardsCompatible(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, []*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
}

func TestDeleteReturnsDbError(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestDeleteReturnsDbErrorBackwardsCompatible(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentResource(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrResourceNotFound)
	repo.AssertExpectations(t)
}

func TestDeleteResource(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id)).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestDeleteResourceBackwardsCompatible(t *testing.T) {
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id)).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestCreateResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Create", mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	// Create the resource
	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Create the same resource again, should not return an error since persistence is disabled
	r, err = useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterData")
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Create")
}

func TestUpdateResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterData")
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Update")
	repo.AssertNotCalled(t, "Create")
}

func TestDeleteResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uint64)(33)).Return(&model.Resource{}, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Delete")
}
