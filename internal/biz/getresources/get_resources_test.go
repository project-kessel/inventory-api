package getresources

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

func (r *MockedResourceRepository) FindByID(ctx context.Context, resourceId uint64) (*model.Resource, error) {
	args := r.Called(ctx, resourceId)
	return args.Get(0).(*model.Resource), args.Error(1)
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

func TestGetResourceById(t *testing.T) {
	resource := resource1()
	repo := &MockedResourceRepository{}

	repo.On("FindByID", mock.Anything, mock.Anything).Return(resource, nil)

	useCase := New(repo, log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.FindById(ctx, 10)
	assert.Nil(t, err)
	assert.Equal(t, resource, r)
}

func TestGetResourceByIdNotFound(t *testing.T) {
	repo := &MockedResourceRepository{}

	repo.On("FindByID", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, log.DefaultLogger)
	ctx := context.TODO()

	r, err := useCase.FindById(ctx, 10)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.Nil(t, r)
}
