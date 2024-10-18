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

type MockedResourceRepository struct {
	mock.Mock
}

func (r *MockedResourceRepository) Save(ctx context.Context, resource *model.Resource) (*model.Resource, error) {
	args := r.Called(ctx, resource)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedResourceRepository) Update(ctx context.Context, resource *model.Resource, id uint64) (*model.Resource, error) {
	args := r.Called(ctx, resource, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedResourceRepository) Delete(ctx context.Context, id uint64) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedResourceRepository) FindByID(ctx context.Context, id uint64) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedResourceRepository) FindByReporterResourceId(ctx context.Context, id model.ReporterResourceId) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedResourceRepository) ListAll(ctx context.Context) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func resource1() *model.Resource {
	return &model.Resource{
		ID:    0,
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

func TestCreateResourceAlreadyExists(t *testing.T) {
	resource := resource1()
	repo := &MockedResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.Error(t, err, "resource already exists")
	repo.AssertExpectations(t)
}

func TestCreateNewResource(t *testing.T) {
	resource := resource1()
	repo := &MockedResourceRepository{}
	returnedResource := model.Resource{
		ID: 10,
	}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedResource, nil)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestUpdateNewResourceCreatesIt(t *testing.T) {
	resource := resource1()
	repo := &MockedResourceRepository{}
	returnedResource := model.Resource{
		ID: 10,
	}

	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Save", mock.Anything, mock.Anything).Return(&returnedResource, nil)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestUpdateExistingResource(t *testing.T) {
	resource := resource1()
	repo := &MockedResourceRepository{}
	returnedResource := model.Resource{
		ID: 10,
	}

	// Resource does not exist
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentResource(t *testing.T) {
	repo := &MockedResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Error(t, err, "resource not found")
	repo.AssertExpectations(t)
}

func TestDeleteResource(t *testing.T) {
	repo := &MockedResourceRepository{}
	ctx := context.TODO()

	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: 33,
	}, nil)
	repo.On("Delete", mock.Anything, (uint64)(33)).Return(&model.Resource{}, nil)

	useCase := New(repo, nil, nil, "", log.DefaultLogger)

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}
