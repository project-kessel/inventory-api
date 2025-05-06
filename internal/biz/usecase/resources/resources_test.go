package resources

import (
	"context"
	"errors"
	"io"
	"sort"
	"testing"

	"google.golang.org/grpc/metadata"

	"github.com/project-kessel/inventory-api/internal/mocks"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/pubsub"
)

type MockedReporterResourceRepository struct {
	mock.Mock
}
type MockedInventoryResourceRepository struct {
	mock.Mock
}

type MockedListenManager struct {
	mock.Mock
}

type MockedSubscription struct {
	mock.Mock
}

type MockLookupResourcesStream struct {
	mock.Mock
	responses []*v1beta1.LookupResourcesResponse
	current   int
}

func (m *MockLookupResourcesStream) Recv() (*v1beta1.LookupResourcesResponse, error) {
	if m.current >= len(m.responses) {
		return nil, io.EOF
	}
	res := m.responses[m.current]
	m.current++
	return res, nil
}

func (m *MockLookupResourcesStream) Header() (metadata.MD, error) {
	args := m.Called()
	return args.Get(0).(metadata.MD), args.Error(1)
}

func (m *MockLookupResourcesStream) Trailer() metadata.MD {
	args := m.Called()
	return args.Get(0).(metadata.MD)
}

func (m *MockLookupResourcesStream) CloseSend() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLookupResourcesStream) Context() context.Context {
	args := m.Called()
	return args.Get(0).(context.Context)
}

func (m *MockLookupResourcesStream) SendMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func (m *MockLookupResourcesStream) RecvMsg(msg interface{}) error {
	args := m.Called(msg)
	return args.Error(0)
}

func TestLookupResources_Success(t *testing.T) {
	ctx := context.TODO()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
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
	mockStream := &MockLookupResourcesStream{
		responses: mockResponses,
	}
	mockStream.On("Recv").Return(mockResponses[0], nil).Once()
	mockStream.On("Recv").Return(mockResponses[1], nil).Once()
	mockStream.On("Recv").Return(nil, io.EOF).Once()
	mockStream.On("Context").Return(ctx)

	// Set up authz mock
	authz.On("LookupResources", ctx, req).Return(mockStream, nil)

	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, false, nil, true, []string{})
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

func (r *MockedReporterResourceRepository) Create(ctx context.Context, resource *model.Resource, namespace string, txid string) (*model.Resource, error) {
	args := r.Called(ctx, resource, namespace, txid)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Update(ctx context.Context, resource *model.Resource, id uuid.UUID, namespace string, txid string) (*model.Resource, error) {
	args := r.Called(ctx, resource, id, namespace, txid)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) Delete(ctx context.Context, id uuid.UUID, namespace string) (*model.Resource, error) {
	args := r.Called(ctx, id, namespace)
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

func (r *MockedReporterResourceRepository) FindByInventoryIdAndReporter(ctx context.Context, inventoryId *uuid.UUID, reporterResourceId string, reporterType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, reporterResourceId, reporterType)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByReporterResourceIdv1beta2(ctx context.Context, id model.ReporterResourceUniqueIndex) (*model.Resource, error) {
	args := r.Called(ctx, id)
	return args.Get(0).(*model.Resource), args.Error(1)
}

func (r *MockedReporterResourceRepository) FindByInventoryIdAndResourceType(ctx context.Context, inventoryId *uuid.UUID, resourceType string) (*model.Resource, error) {
	args := r.Called(ctx, inventoryId, resourceType)
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

func (r *MockedReporterResourceRepository) FindByWorkspaceId(ctx context.Context, workspace_id string) ([]*model.Resource, error) {
	args := r.Called(ctx)
	return args.Get(0).([]*model.Resource), args.Error(1)
}

func (m *MockedListenManager) Subscribe(txid string) pubsub.Subscription {
	args := m.Called(txid)
	return args.Get(0).(pubsub.Subscription)
}

func (m *MockedListenManager) WaitAndDistribute(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockedListenManager) Run(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockedSubscription) NotificationC() <-chan []byte {
	args := m.Called()
	return args.Get(0).(chan []byte)
}

func (m *MockedSubscription) Unsubscribe() {
	m.Called()
}

func (m *MockedSubscription) BlockForNotification(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
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

func TestCreateReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, listenMan, false, []string{})
	ctx := context.TODO()

	r, err := useCase.Create(ctx, resource)
	assert.Nil(t, err)
	assert.Equal(t, &returnedResource, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}

func TestCreateNewResource_ConsistencyToken(t *testing.T) {
	// TODO: Follow up with leads on which consistency to support in v1beta1
	// TODO: Check that consistency token is actually updated
	resource := resource1()
	id, err := uuid.NewV7()
	assert.Nil(t, err)

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	m := &mocks.MockAuthz{}
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}

	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, listenMan, true, []string{"reporter_id"})
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
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, listenMan, false, []string{})
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

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}
	returnedResource := model.Resource{
		ID: id,
	}

	// Resource already exists
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, listenMan, false, []string{})
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

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, listenMan, false, []string{})
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
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// Validates backwards compatibility
	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
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
	repo.On("Delete", mock.Anything, (uuid.UUID)(id), mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})

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
	repo.On("Delete", mock.Anything, (uuid.UUID)(id), mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})

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
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence, nil, false, []string{})

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
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)

	disablePersistence := true
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence, nil, false, []string{})

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

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}

	returnedResource := model.Resource{
		ID: id,
	}

	repo.On("FindByReporterData", mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&returnedResource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, false, listenMan, true, []string{"reporter_id"})
	ctx := context.TODO()

	r, err := useCase.Update(ctx, resource, model.ReporterResourceId{})

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
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
	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, disablePersistence, nil, false, []string{})

	err = useCase.Delete(ctx, model.ReporterResourceId{})
	assert.Nil(t, err)

	// Assert that the repository methods were not called since persistence is disabled
	repo.AssertNotCalled(t, "FindByReporterResourceId")
	repo.AssertNotCalled(t, "Delete")
}

func TestCheck_MissingResource(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.Check(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheck_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrUnsupportedDriver) // some random error

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_ErrorWithKessel(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, errors.New("failed during call to relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	assert.NotNil(t, err)
	assert.False(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_WorkspaceAllowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{ResourceType: "workspace"})

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)
}

func TestCheckForUpdate_MissingResource_Allowed(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(&model.Resource{}, gorm.ErrRecordNotFound)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	allowed, err := useCase.CheckForUpdate(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, model.ReporterResourceId{})

	// no consistency token being written.

	assert.Nil(t, err)
	assert.True(t, allowed)

	repo.AssertExpectations(t)

}

func TestCheckForUpdate_Allowed(t *testing.T) {
	ctx := context.TODO()
	resource := resource1()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByReporterResourceId", mock.Anything, mock.Anything).Return(resource, nil)
	m.On("CheckForUpdate", mock.Anything, mock.Anything, "notifications_integration_view", mock.Anything, mock.Anything).Return(v1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, errors.New("failed querying"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.NotNil(t, err)
	assert.Nil(t, resource_chan)
	assert.Nil(t, err_chan)

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_NoResources(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
	resource_chan, err_chan, err := useCase.ListResourcesInWorkspace(ctx, "notifications_integration_view", "rbac", &v1beta1.SubjectReference{}, "foo-id")

	assert.Nil(t, err)

	res := <-resource_chan
	assert.Nil(t, res) // expecting no resources

	assert.Empty(t, err_chan) // dont want any errors.

	repo.AssertExpectations(t)
}

func TestListResourcesInWorkspace_ResourcesAllowedTrue(t *testing.T) {
	ctx := context.TODO()

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()
	resource2 := resource2()
	resource3 := resource3()
	theError := errors.New("failed calling relations")

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource, resource2, resource3}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_FALSE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource2, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, nil)
	m.On("Check", mock.Anything, mock.Anything, "notifications_integration_write", resource3, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_UNSPECIFIED, &v1beta1.ConsistencyToken{}, theError)

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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

	inventoryRepo := &MockedInventoryResourceRepository{}
	repo := &MockedReporterResourceRepository{}
	m := &mocks.MockAuthz{}

	resource := resource1()

	repo.On("FindByWorkspaceId", mock.Anything, mock.Anything).Return([]*model.Resource{resource}, nil)
	m.On("Check", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(v1beta1.CheckResponse_ALLOWED_TRUE, &v1beta1.ConsistencyToken{}, errors.New("failed calling relations"))

	useCase := New(repo, inventoryRepo, m, nil, "", log.DefaultLogger, false, nil, false, []string{})
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
		waitForSync             bool
		ReadAfterWriteEnabled   bool
		ReadAfterWriteAllowlist []string
		expected                bool
	}{
		{
			name:                    "Enable Read After Write, Wait for Sync, SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             true,
			ReadAfterWriteAllowlist: []string{"SP1"},
			expected:                true,
		},
		{
			name:                    "Enable Read After Write, No Wait for Sync, SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             false,
			ReadAfterWriteAllowlist: []string{"SP1"},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, ALL SPs in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             true,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                true,
		},
		{
			name:                    "Enable Read After Write, No Wait for Sync, ALL SPs in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             false,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, No SP in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             true,
			ReadAfterWriteAllowlist: []string{},
			expected:                false,
		},
		{
			name:                    "Enable Read After Write, Wait for Sync, SP not in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   true,
			waitForSync:             true,
			ReadAfterWriteAllowlist: []string{"SP2"},
			expected:                false,
		},
		{
			name:                    "Disable Read After Write, No Wait for Sync, SP not in Allowlist",
			listenManager:           listenManager,
			ReadAfterWriteEnabled:   false,
			waitForSync:             false,
			ReadAfterWriteAllowlist: []string{"SP2"},
			expected:                false,
		},
		{
			name:                    "Nil ListenManager, Enabled Read After Write, Wait for Sync, SP in Allowlist",
			listenManager:           listenManagerNil,
			ReadAfterWriteEnabled:   true,
			waitForSync:             true,
			ReadAfterWriteAllowlist: []string{"*"},
			expected:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &Usecase{
				ListenManager:           tt.listenManager,
				ReadAfterWriteEnabled:   tt.ReadAfterWriteEnabled,
				ReadAfterWriteAllowlist: tt.ReadAfterWriteAllowlist,
			}

			m := &model.Resource{
				ReporterId: "SP1",
			}
			assert.Equal(t, tt.expected, computeReadAfterWrite(uc, tt.waitForSync, m))

		})
	}
}

func TestUpsertReturnsDbError(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// DB Error
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrDuplicatedKey)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
	ctx := context.TODO()

	_, err := useCase.Upsert(ctx, resource, false)
	assert.ErrorIs(t, err, ErrDatabaseError)
	repo.AssertExpectations(t)
}

func TestUpsertReturnsExistingUpdatedResource(t *testing.T) {
	resource := resource1()
	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}

	// No Error
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return(resource, nil)
	repo.On("Update", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	useCase := New(repo, inventoryRepo, nil, nil, "", log.DefaultLogger, false, nil, false, []string{})
	ctx := context.TODO()

	res, err := useCase.Upsert(ctx, resource, false)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	repo.AssertExpectations(t)
}

func TestUpsert_ReadAfterWrite(t *testing.T) {
	resource := resource1()

	repo := &MockedReporterResourceRepository{}
	inventoryRepo := &MockedInventoryResourceRepository{}
	authz := &mocks.MockAuthz{}
	listenMan := &MockedListenManager{}
	sub := MockedSubscription{}

	// no existing resource, need to create
	repo.On("FindByReporterResourceIdv1beta2", mock.Anything, mock.Anything).Return((*model.Resource)(nil), gorm.ErrRecordNotFound)
	repo.On("Create", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resource, nil)

	listenMan.On("Subscribe", mock.Anything).Return(&sub)

	sub.On("Unsubscribe")
	sub.On("BlockForNotification", mock.Anything).Return(nil)

	useCase := New(repo, inventoryRepo, authz, nil, "", log.DefaultLogger, false, listenMan, true, []string{"reporter_id"})
	ctx := context.TODO()

	r, err := useCase.Upsert(ctx, resource, true)

	assert.Nil(t, err)
	assert.NotNil(t, r)
	repo.AssertExpectations(t)
	listenMan.AssertExpectations(t)
	sub.AssertExpectations(t)
}
