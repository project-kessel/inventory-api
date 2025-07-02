package resources

import (
	"context"
	"errors"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/mocks"
	"github.com/sony/gobreaker"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"
)

func TestLookupResources_Success(t *testing.T) {
	ctx := context.TODO()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}

	req := &v1beta1.LookupResourcesRequest{
		ResourceType: &v1beta1.ObjectType{
			Namespace: "test-namespace",
			Name:      "test-resource",
		},
		Relation: "view",
		Subject: &v1beta1.SubjectReference{
			Subject: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "user",
					Name:      "default",
				},
				Id: "user1",
			},
		},
	}

	mockResponses := []*v1beta1.LookupResourcesResponse{
		{
			Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "test-namespace",
					Name:      "test-resource",
				},
				Id: "resource1",
			},
		},
		{
			Resource: &v1beta1.ObjectReference{
				Type: &v1beta1.ObjectType{
					Namespace: "test-namespace",
					Name:      "test-resource",
				},
				Id: "resource2",
			},
		},
	}

	// Set up mock stream
	mockStream := &mocks.MockLookupResourcesStream{
		Responses: mockResponses,
	}
	mockStream.On("Recv").Return(mockResponses[0], nil).Once()
	mockStream.On("Recv").Return(mockResponses[1], nil).Once()
	mockStream.On("Recv").Return(nil, io.EOF).Once()
	mockStream.On("Context").Return(ctx)

	// Set up authz mock
	authz.On("LookupResources", ctx, req).Return(mockStream, nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, nil, cb, usecaseConfig)
	stream, err := useCase.LookupResources(ctx, req)

	assert.Nil(t, err)
	assert.NotNil(t, stream)

	// Verify we can receive all responses
	res1, err := stream.Recv()
	assert.Nil(t, err)
	assert.Equal(t, "resource1", res1.Resource.Id)

	res2, err := stream.Recv()
	assert.Nil(t, err)
	assert.Equal(t, "resource2", res2.Resource.Id)

	// Verify EOF
	_, err = stream.Recv()
	assert.Equal(t, io.EOF, err)
}

func resource1() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org",
		ResourceData: map[string]any{
			"foo": "bar",
		},
		ReporterId:   "reporter_id",
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

func resource2() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org2",
		ResourceData: map[string]any{
			"foo2": "bar2",
		},
		ResourceType: "my-resource2",
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
				Key:   "label-2",
				Value: "value-2",
			},
			{
				Key:   "label-2",
				Value: "value-3",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

func resource3() *model.Resource {
	return &model.Resource{
		ID:    uuid.UUID{},
		OrgId: "my-org3",
		ResourceData: map[string]any{
			"foo3": "bar3",
		},
		ResourceType: "my-resource33",
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
				Key:   "label-3",
				Value: "value-3",
			},
			{
				Key:   "label-2",
				Value: "value-3",
			},
			{
				Key:   "label-xyz",
				Value: "value-xyz",
			},
		},
	}
}

var defaultUseCaseConfig = &UsecaseConfig{
	DisablePersistence:      false,
	ReadAfterWriteEnabled:   false,
	ReadAfterWriteAllowlist: []string{},
	ConsumerEnabled:         true,
}
var cb = gobreaker.NewCircuitBreaker(gobreaker.Settings{
	Name:    "wait-for-notif-breaker",
	Timeout: 1 * time.Second,
	ReadyToTrip: func(counts gobreaker.Counts) bool {
		// Trip after 3 consecutive failures
		return counts.ConsecutiveFailures > 2
	},
})

func TestCreateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Validates backwards compatibility, record was not found via new method
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExists(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateResourceAlreadyExistsBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Create(ctx, resource)
	assert.ErrorIs(t, err, ErrResourceAlreadyExists)
	repo.AssertExpectations(t)
}

func TestCreateNewResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, listenMan, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestCreateNewResource_ConsumerDisabled(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   false,
		ReadAfterWriteAllowlist: []string{},
		ConsumerEnabled:         false,
	}
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
	listenMan.AssertNotCalled(t, "Subscribe")
	sub.AssertNotCalled(t, "Unsubscribe")
	sub.AssertNotCalled(t, "BlockForNotification")
}

func TestCreateNewResource_ConsistencyToken(t *testing.T) {
	// TODO: Follow up with leads on which consistency to support in v1beta1
	// TODO: Check that consistency token is actually updated
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	m := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestUpdateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestUpdateReturnsDbErrorBackwardsCompatible(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestUpdateNewResourceCreatesIt(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource doesn't exist
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, listenMan, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestUpdateExistingResource(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, listenMan, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}
func TestUpdateExistingResourceBackwardsCompatible(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	resource.ID = id

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, listenMan, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	assert.Equal(t, resource.ID, r.ID)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestDeleteReturnsDbError(t *testing.T) {
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}
func TestDeleteReturnsDbErrorBackwardsCompatible(t *testing.T) {
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// DB Error
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestDeleteNonexistentResource(t *testing.T) {
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	err := useCase.Delete(ctx, model.ReporterResourceId{})
	assert.ErrorIs(t, err, ErrResourceNotFound)
	repo.AssertExpectations(t)
}

func TestDeleteResource(t *testing.T) {
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id), mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestDeleteResourceBackwardsCompatible(t *testing.T) {
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	ctx := context.TODO()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	// Resource already exists
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uuid.UUID)(id), mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	repo.AssertExpectations(t)
}

func TestCreateResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      true,
		ReadAfterWriteEnabled:   false,
		ReadAfterWriteAllowlist: []string{},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, usecaseConfig)

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
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      true,
		ReadAfterWriteEnabled:   false,
		ReadAfterWriteAllowlist: []string{},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, usecaseConfig)

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})
	assert.Nil(t, err)
	assert.Equal(t, resource, r)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterData")
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Update")
	repo.AssertNotCalled(t, "Create")
}

func TestUpdate_ReadAfterWrite(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestUpdate_ConsumerDisabled(t *testing.T) {
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         false,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertNotCalled(t, "Subscribe")
	sub.AssertNotCalled(t, "Unsubscribe")
	sub.AssertNotCalled(t, "BlockForNotification")
}

func TestDeleteResource_PersistenceDisabled(t *testing.T) {
	ctx := context.TODO()

	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// Mock as if persistence is not disabled, for assurance
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{
		ID: id,
	}, nil)
	repo.On("Delete", mock.Anything, (uint64)(33)).Return(&model.Resource{}, nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      true,
		ReadAfterWriteEnabled:   false,
		ReadAfterWriteAllowlist: []string{},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, usecaseConfig)

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Delete")
}

func TestCheck_MissingResource(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	allowed, err = useCase.Check(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_ResourceExistsError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.Check(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	allowed, err = useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_ResourceExistsError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_WorkspaceAllowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{ResourceType: "workspace"})

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_MissingResource_Allowed(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	// no consistency token being written.

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)

}

func TestCheckForUpdate_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.True(t, allowed)

	// check negative case
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)

	allowed, err = useCase.CheckForUpdate(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_Error(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, errors.New("failed querying"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.NotNil(t, err)
	assert.Nil(t, resource_chan)
	assert.Nil(t, err_chan)

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_NoResources(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Nil(t, res) // expecting no resources

	assert.Empty(t, err_chan) // dont want any errors.

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_ResourcesAllowedTrue(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Equal(t, resource, res) // expecting to get back resource1

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed")
	}

	assert.Empty(t, err_chan) // dont want any errors.

	// check negative case (not allowed)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	resource_chan, err_chan, err = useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res = <-resource_chan
	assert.Nil(t, res) // expecting no resource, as we are not allowed

	assert.Empty(t, err_chan) // dont want any errors.

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_MultipleResourcesAllowedTrue(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	out := make([]*model.Resource, 3)
	out[0] = <-resource_chan
	out[1] = <-resource_chan
	out[2] = <-resource_chan

	in := []*model.Resource{resource, resource2, resource3}
	sort.Slice(in, func(i, j int) bool { return len(in[i].ResourceType) < len(in[j].ResourceType) })
	sort.Slice(out, func(i, j int) bool { return len(out[i].ResourceType) < len(out[j].ResourceType) })
	assert.Equal(t, in, out) // all 3 are there in any order

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed") // and there was no other resource
	}

	assert.Empty(t, err_chan) // dont want any errors.
}

// not authorized for the middle one and error on the third should just pass first
func TestListResourcesInWorkspace_MultipleResourcesOneFalseTwoTrueLastError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()
	theError := errors.New("failed calling relations")

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource2, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource3, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_UNSPECIFIED, &v1beta1.ConsistencyToken{}, theError)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_write", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	out_allowed := make([]*model.Resource, 1)
	out_allowed[0] = <-resource_chan

	in_allowed := []*model.Resource{resource2}
	sort.Slice(in_allowed, func(i, j int) bool { return len(in_allowed[i].ResourceType) < len(in_allowed[j].ResourceType) })
	sort.Slice(out_allowed, func(i, j int) bool { return len(out_allowed[i].ResourceType) < len(out_allowed[j].ResourceType) })
	assert.Equal(t, in_allowed, out_allowed) // all 3 are there in any order

	_, ok := <-resource_chan
	if ok {
		t.Error("resource_chan should have been closed") // and there was no other resource
	}

	backError := <-err_chan
	assert.Equal(t, theError, backError) // dont want any errors.
}

func TestListResourcesInWorkspace_ResourcesAllowedError(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	repo := &mocks.MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, errors.New("failed calling relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Nil(t, res) // expecting no resource, as we errored

	assert.NotEmpty(t, err_chan) // we want an errors.
}

func TestIsSPInAllowlist(t *testing.T) {
	tests := []struct {
		name      string
		resource  *model.Resource
		allowlist []string
		expected  bool
	}{
		{
			name:      "SP in allowlist",
			resource:  &model.Resource{ReporterId: "sp1"},
			allowlist: []string{"sp1", "sp2"},
			expected:  true,
		},
		{
			name:      "SP not in allowlist",
			resource:  &model.Resource{ReporterId: "sp3"},
			allowlist: []string{"sp1", "sp2"},
			expected:  false,
		},
		{
			name:      "Wildcard '*' allows any SP",
			resource:  &model.Resource{ReporterId: "sp3"},
			allowlist: []string{"*"},
			expected:  true,
		},
		{
			name:      "SP in allowlist with wildcard",
			resource:  &model.Resource{ReporterId: "sp3"},
			allowlist: []string{"sp1", "*"},
			expected:  true,
		},
		{
			name:      "Empty allowlist",
			resource:  &model.Resource{ReporterId: "sp1"},
			allowlist: []string{},
			expected:  false,
		},
		{
			name:      "Allowlist with only wildcard",
			resource:  &model.Resource{ReporterId: "sp4"},
			allowlist: []string{"*"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSPInAllowlist(tt.resource, tt.allowlist)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestComputeReadAfterWrite(t *testing.T) {
	var listenManager = &pubsub.ListenManager{}
	var listenManagerNil *pubsub.ListenManager

	tests := []struct {
		name                    string
		listenManager           pubsub.ListenManagerImpl
		writeVisibility         v1beta2.WriteVisibility
		ReadAfterWriteEnabled   bool
		ReadAfterWriteAllowlist []string
		expected                bool
	}{
		{
			name:                    "Enable Read After Write, Wait for Sync, SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_IMMEDIATE,
			ReadAfterWriteAllowlist: []string{"SP1"},
			expected:                true,
		},
		{
			name:                    "Enable Read After Write, No Wait for Sync, SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED,
			ReadAfterWriteAllowlist: []string{"SP1"},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, ALL SPs in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_IMMEDIATE,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                true,
		},
		{
			name:                    "Enable Read After Write, No Wait for Sync, ALL SPs in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Minimize Latency, ALL SPs in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_MINIMIZE_LATENCY,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, No SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_IMMEDIATE,
			ReadAfterWriteAllowlist: []string{},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, SP not in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_IMMEDIATE,
			ReadAfterWriteAllowlist: []string{"SP2"},
			expected:                false,
		},
		{
			name:                    "Disable Read After Write, No Wait for Sync, SP not in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   false,
			writeVisibility:         v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED,
			ReadAfterWriteAllowlist: []string{"SP2"},
			expected:                false,
		},
		{
			name:                    "Nil ListenManager, Enabled Read After Write, Wait for Sync, SP in Allowlist",
			listenManager:           listenManagerNil,
			ReadAfterWriteEnabled:   true,
			writeVisibility:         v1beta2.WriteVisibility_IMMEDIATE,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usecaseConfig := &UsecaseConfig{
				DisablePersistence:      false,
				ReadAfterWriteEnabled:   tt.ReadAfterWriteEnabled,
				ReadAfterWriteAllowlist: tt.ReadAfterWriteAllowlist,
				ConsumerEnabled:         true,
			}
			uc := &Usecase{
				ListenManager: tt.listenManager,
				Config:        usecaseConfig,
			}

			m := &model.Resource{
				ReporterId: "SP1",
			}
			assert.Equal(t, tt.expected, computeReadAfterWrite(uc, tt.writeVisibility, m))

		})
	}
}

func TestUpsertReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	_, err := useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestUpsertReturnsExistingUpdatedResource(t *testing.T) {
	resource := resource1()
	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}

	// No Error
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, nil, cb, defaultUseCaseConfig)
	ctx := context.TODO()

	res, err := useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_WRITE_VISIBILITY_UNSPECIFIED)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	repo.AssertExpectations(t)
}

func TestUpsert_ReadAfterWrite(t *testing.T) {
	resource := resource1()

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	// no existing resource, need to create
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestUpsert_ConsumerDisabled(t *testing.T) {
	resource := resource1()

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	// no existing resource, need to create
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         false,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	r, err := useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertNotCalled(t, "Subscribe")
	sub.AssertNotCalled(t, "Unsubscribe")
	sub.AssertNotCalled(t, "BlockForNotification")
}

func TestUpsert_WaitCircuitBreaker(t *testing.T) {
	resource := resource1()

	repo := &mocks.MockedReporterResourceRepository{}
	inventoryRepo := &mocks.MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &mocks.MockedListenManager{}
	sub := mocks.MockedSubscription{}

	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	// Return timeout error
	blockForNotifCall := sub.On("BlockForNotification", mock.Anything).Return(pubsub.ErrWaitContextCancelled)

	usecaseConfig := &UsecaseConfig{
		DisablePersistence:      false,
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{"reporter_id"},
		ConsumerEnabled:         true,
	}
	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, listenMan, cb, usecaseConfig)
	ctx := context.TODO()

	// Attempt 1 - Trigger failure
	r, err := useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)
	assert.Nil(t, err) // No expected error because we treat a timeout as a success
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
	assert.Equal(t, gobreaker.StateClosed, cb.State()) // Circuit breaker should be closed
	// Attempt 2 - Trigger failure
	r, err = useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)
	assert.Nil(t, err) // No expected error because we treat a timeout as a success
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
	assert.Equal(t, gobreaker.StateClosed, cb.State()) // Circuit breaker should be closed
	// Attempt 3 - Trigger final failure
	r, err = useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)
	assert.Nil(t, err) // No expected error because we treat a timeout as a success
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
	assert.Equal(t, gobreaker.StateOpen, cb.State()) // Circuit breaker should be open
	// Attempt 4 - test open state
	r, err = useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)
	assert.Nil(t, err) // No expected error because we treat a timeout as a success
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
	assert.Equal(t, gobreaker.StateOpen, cb.State()) // Circuit breaker should still be open
	// Circuit breaker reset
	blockForNotifCall.Unset()
	time.Sleep(2 * time.Second)                               // Wait for the circuit breaker timeout
	assert.Equal(t, gobreaker.StateHalfOpen, cb.State())      // Circuit breaker should be half-open after the timeout
	sub.On("BlockForNotification", mock.Anything).Return(nil) // Prepare a successful notification
	// Attempt 5 - test half-open state returned to closed
	r, err = useCase.Upsert(ctx, resource, v1beta2.WriteVisibility_IMMEDIATE)
	assert.Nil(t, err) // No expected error because we treat a timeout as a success
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
	assert.Equal(t, gobreaker.StateClosed, cb.State()) // Circuit breaker should be closed
}
