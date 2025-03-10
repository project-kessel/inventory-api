package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

type ResourceService struct {
	pb.UnimplementedKesselResourceServiceServer
	Ctl *resources.Usecase
}

func New(c *resources.Usecase) *ResourceService {
	return &ResourceService{
		Ctl: c,
	}
}

func (c *ResourceService) ReportResource(ctx context.Context, r *pb.ReportResourceRequest) (*pb.ReportResourceResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	resource, err := requestToResource(r, identity)
	if err != nil {
		return nil, err
	}
	_, err = c.Ctl.Upsert(ctx, resource)
	log.Info()
	if err != nil {
		return nil, err
	}
	return responseFromResource(), nil
}

func (c *ResourceService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	log.Info("I am in the new Resource Service Delete method!", ctx, r)
	return nil, nil
}

func requestToResource(r *pb.ReportResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {
	log.Info("Report Resource Request: ", r)
	var resourceType = r.Resource.GetResourceType()
	resourceData, err := conv.ToJsonObject(r.Resource.ReporterData.ResourceData)
	if err != nil {
		return nil, err
	}

	var workspaceId, err2 = conv.ExtractWorkspaceId(r.Resource.CommonResourceData)
	if err2 != nil {
		return nil, err2
	}
	return conv.ResourceFromPb(resourceType, identity.Principal, resourceData, workspaceId, r.Resource.ReporterData), nil
}

func responseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}
