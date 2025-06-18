package resources_test

import (
	"context"
	"regexp"

	"io"
	"testing"

	krlog "github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	relationsV1beta1 "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	usecase "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/inventory-api/internal/mocks"
	svc "github.com/project-kessel/inventory-api/internal/service/resources"
)

type MockRepo = mocks.MockedReporterResourceRepository

func TestRequestToResource_Success(t *testing.T) {
	invID := "9b8b5a02-4ac7-4d6c-a2c0-123456789abc"
	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "inst-1",
		InventoryId:        &invID,
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-123"),
				},
			},
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.NoError(t, err)
	assert.NotNil(t, resource)
	assert.Equal(t, "host", req.GetType())
	assert.Equal(t, "hbi", req.GetReporterType())
	assert.Equal(t, "inst-1", req.GetReporterInstanceId())
	assert.Equal(t, "9b8b5a02-4ac7-4d6c-a2c0-123456789abc", req.GetInventoryId())
}

func TestRequestToResource_Error_InvalidInventoryID(t *testing.T) {
	id := "bad-uuid"
	req := &pb.ReportResourceRequest{
		InventoryId: &id,
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "invalid inventory ID")
}

func TestRequestToResource_Error_MissingReporterType(t *testing.T) {
	req := &pb.ReportResourceRequest{
		ReporterType:       "",
		ReporterInstanceId: "inst",
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "reporterType")
}

func TestRequestToResource_Error_MissingReporterInstanceId(t *testing.T) {
	req := &pb.ReportResourceRequest{
		ReporterType:       "hbi",
		ReporterInstanceId: "",
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws"),
				},
			},
		},
	}
	identity := &authnapi.Identity{Principal: "reporter-1"}

	resource, err := svc.RequestToResource(req, identity)

	assert.Error(t, err)
	assert.Nil(t, resource)
	assert.Contains(t, err.Error(), "reporterInstanceId")
}

func TestInventoryService_ReportResource_MissingReporterType(t *testing.T) {
	id := &authnapi.Identity{Principal: "tester", Type: "reporterType"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := &usecase.Usecase{
		Log: krlog.NewHelper(krlog.NewStdLogger(io.Discard)),
	}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestInventoryService_ReportResource_MissingReporterInstanceId(t *testing.T) {
	id := &authnapi.Identity{Principal: "tester", Type: "reporterType"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "",
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"hostname": structpb.NewStringValue("example-host"),
				},
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := &usecase.Usecase{
		Log: krlog.NewHelper(krlog.NewStdLogger(io.Discard)),
	}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "reporterInstanceId")
}

func TestInventoryService_ReportResource_InvalidJsonObject(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	badCommon := &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"invalid": nil, // use intentionally unsupported types
		},
	}

	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		Representations: &pb.ResourceRepresentations{
			Common: badCommon,
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := &usecase.Usecase{
		Log: krlog.NewHelper(krlog.NewStdLogger(io.Discard)),
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestInventoryService_ReportResource_InvalidInventoryId(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	badUUID := "not-a-uuid"
	req := &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "instance-001",
		InventoryId:        &badUUID,
		Representations: &pb.ResourceRepresentations{
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue("ws-001"),
				},
			},
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: "v1",
			},
			Reporter: &structpb.Struct{},
		},
	}

	uc := &usecase.Usecase{
		Log: krlog.NewHelper(krlog.NewStdLogger(io.Discard)),
	}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.ReportResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "invalid inventory ID")
}

func TestResponseFromResource(t *testing.T) {
	resp := svc.ResponseFromResource()

	assert.NotNil(t, resp)
	assert.IsType(t, &pb.ReportResourceResponse{}, resp)
	assert.Equal(t, &pb.ReportResourceResponse{}, resp)
}

func TestInventoryService_DeleteResource_Success(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceId:   "abc123",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
	}

	mockRepo := new(MockRepo)

	mockRepo.
		On("FindByReporterData", mock.Anything, "sarah", "abc123").
		Return(&model.Resource{}, nil).
		Once()

	mockRepo.
		On("Delete",
			mock.AnythingOfType("*context.valueCtx"),
			mock.AnythingOfType("uuid.UUID"),
			"rbac",
		).
		Return(&model.Resource{
			ID:           uuid.New(),
			ResourceType: "host",
		}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{}
	uc := &usecase.Usecase{
		ReporterResourceRepository: mockRepo,
		Config:                     cfg,
		Namespace:                  "rbac",
		Log:                        krlog.NewHelper(krlog.NewStdLogger(io.Discard)),
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.DeleteResource(ctx, req)

	assert.NoError(t, err)
	assert.NotNil(t, resp)

	mockRepo.AssertExpectations(t)
}

func TestInventoryService_DeleteResource_NoIdentity(t *testing.T) {
	ctx := context.Background() // no identity

	req := &pb.DeleteResourceRequest{}

	uc := &usecase.Usecase{}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.DeleteResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to get identity")
}

func TestInventoryService_DeleteResource_InvalidRequest(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.DeleteResourceRequest{
		Reference: nil, // invalid
	}

	uc := &usecase.Usecase{}
	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.DeleteResource(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "failed to build reporter resource ID")
}

func TestInventoryService_Check_Allowed(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "sarah",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	mockRepo := new(MockRepo)
	mockRepo.
		On("FindByReporterResourceId", mock.Anything, mock.Anything).
		Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	mockAuthz := new(mocks.MockAuthz)
	mockAuthz.
		On("Check",
			mock.Anything,
			"hbi",
			"view",
			mock.AnythingOfType("*model.Resource"),
			mock.Anything,
		).
		Return(relationsV1beta1.CheckResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{DisablePersistence: true}
	uc := &usecase.Usecase{
		Authz:                      mockAuthz,
		ReporterResourceRepository: mockRepo,
		Namespace:                  "rbac",
		Config:                     cfg,
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.Check(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
	}

	mockRepo.AssertExpectations(t)
	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckForUpdate_Allowed(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckForUpdateRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "sarah",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	mockRepo := new(MockRepo)
	mockRepo.
		On("FindByReporterResourceId", mock.Anything, mock.Anything).
		Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	mockAuthz := new(mocks.MockAuthz)
	mockAuthz.
		On("CheckForUpdate",
			mock.Anything,
			"hbi",
			"view",
			mock.AnythingOfType("*model.Resource"),
			mock.Anything,
		).
		Return(relationsV1beta1.CheckForUpdateResponse_ALLOWED_TRUE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{DisablePersistence: true}
	uc := &usecase.Usecase{
		Authz:                      mockAuthz,
		ReporterResourceRepository: mockRepo,
		Namespace:                  "rbac",
		Config:                     cfg,
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckForUpdate(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.Allowed)
	}

	mockRepo.AssertExpectations(t)
	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_Check_Denied(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "sarah",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	mockRepo := new(MockRepo)
	mockRepo.
		On("FindByReporterResourceId", mock.Anything, mock.Anything).
		Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	mockAuthz := new(mocks.MockAuthz)
	mockAuthz.
		On("Check",
			mock.Anything,
			"hbi",
			"view",
			mock.AnythingOfType("*model.Resource"),
			mock.Anything,
		).
		Return(relationsV1beta1.CheckResponse_ALLOWED_FALSE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{DisablePersistence: true}
	uc := &usecase.Usecase{
		Authz:                      mockAuthz,
		ReporterResourceRepository: mockRepo,
		Namespace:                  "rbac",
		Config:                     cfg,
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.Check(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
	}

	mockRepo.AssertExpectations(t)
	mockAuthz.AssertExpectations(t)
}

func TestInventoryService_CheckForUpdate_Denied(t *testing.T) {
	id := &authnapi.Identity{Principal: "sarah", Type: "rbac"}
	ctx := context.WithValue(context.Background(), middleware.IdentityRequestKey, id)

	req := &pb.CheckForUpdateRequest{
		Relation: "view",
		Object: &pb.ResourceReference{
			ResourceId:   "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
			ResourceType: "host",
			Reporter:     &pb.ReporterReference{Type: "hbi"},
		},
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceId:   "sarah",
				ResourceType: "principal",
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	mockRepo := new(MockRepo)
	mockRepo.
		On("FindByReporterResourceId", mock.Anything, mock.Anything).
		Return((*model.Resource)(nil), gorm.ErrRecordNotFound)

	mockAuthz := new(mocks.MockAuthz)
	mockAuthz.
		On("CheckForUpdate",
			mock.Anything,
			"hbi",
			"view",
			mock.AnythingOfType("*model.Resource"),
			mock.Anything,
		).
		Return(relationsV1beta1.CheckForUpdateResponse_ALLOWED_FALSE, &relationsV1beta1.ConsistencyToken{}, nil).
		Once()

	cfg := &usecase.UsecaseConfig{DisablePersistence: true}
	uc := &usecase.Usecase{
		Authz:                      mockAuthz,
		ReporterResourceRepository: mockRepo,
		Namespace:                  "rbac",
		Config:                     cfg,
	}

	service := svc.NewKesselInventoryServiceV1beta2(uc)

	resp, err := service.CheckForUpdate(ctx, req)

	assert.NoError(t, err)
	if assert.NotNil(t, resp) {
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.Allowed)
	}

	mockRepo.AssertExpectations(t)
	mockAuthz.AssertExpectations(t)
}

func TestToLookupResourceRequest(t *testing.T) {
	permission := "view"
	input := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "hbi",
		},
		Relation: "view",
		Subject: &pb.SubjectReference{
			Relation: &permission,
			Resource: &pb.ResourceReference{
				ResourceId:   "res-id",
				ResourceType: "principal",
				Reporter: &pb.ReporterReference{
					Type: "rbac",
				},
			},
		},
		Pagination: &pb.RequestPagination{
			Limit: 50,
		},
	}

	expected := &relationsV1beta1.LookupResourcesRequest{
		ResourceType: &relationsV1beta1.ObjectType{
			Name: "hbi",
		},
		Relation: "view",
		Subject: &relationsV1beta1.SubjectReference{
			Relation: &permission,
			Subject: &relationsV1beta1.ObjectReference{
				Type: &relationsV1beta1.ObjectType{
					Name:      "principal",
					Namespace: "rbac",
				},
				Id: "res-id",
			},
		},
		Pagination: &relationsV1beta1.RequestPagination{
			Limit: 50,
		},
	}

	result, _ := svc.ToLookupResourceRequest(input)
	assert.Equal(t, expected, result)
}

func TestIsValidatedRepresentationType(t *testing.T) {

	assert.True(t, IsValidType("hbi"))

	// normalize then validate
	normalized := svc.NormalizeType("HBI")
	assert.True(t, IsValidType(normalized))
	// strange characters
	assert.False(t, IsValidType("h?!!!"))
}

func TestNormalizeRepresentationType(t *testing.T) {
	// normalize then validate
	normalized := svc.NormalizeType("HBI")
	assert.True(t, IsValidType(normalized))

	assert.Equal(t, "hbi", normalized)
}

var typePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func IsValidType(val string) bool {
	return typePattern.MatchString(val)
}

func TestToLookupResourceResponse(t *testing.T) {
	input := &relationsV1beta1.LookupResourcesResponse{
		Resource: &relationsV1beta1.ObjectReference{
			Type: &relationsV1beta1.ObjectType{
				Namespace: "reporter-x",
				Name:      "type-y",
			},
			Id: "abc123",
		},
		Pagination: &relationsV1beta1.ResponsePagination{
			ContinuationToken: "next-page-token",
		},
	}

	expected := &pb.StreamedListObjectsResponse{
		Object: &pb.ResourceReference{
			Reporter: &pb.ReporterReference{
				Type: "reporter-x",
			},
			ResourceId: "abc123",
		},
		Pagination: &pb.ResponsePagination{
			ContinuationToken: "next-page-token",
		},
	}

	result := svc.ToLookupResourceResponse(input)
	assert.Equal(t, expected, result)
}
