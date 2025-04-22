package resources

import (
	"context"
	"fmt"
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

func NewKesselResourceServiceV1beta2(c *resources.Usecase) *ResourceService {
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

// DeleteResource NOT Deleting the correct resources
func (c *ResourceService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	log.Info("I am in the new Resource Service Delete method!", ctx, r)

	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get identity: %w", err)
	}

	reporterResource, err := requestToDeleteResource(r, identity)
	if err != nil {
		log.Error("Failed to build reporter resource ID: ", err)
		return nil, fmt.Errorf("failed to build reporter resource ID: %w", err)
	}

	err = c.Ctl.Delete(ctx, reporterResource)
	if err != nil {
		log.Error("Failed to delete resource: ", err)
		return nil, fmt.Errorf("failed to delete resource: %w", err)
	}

	return responseFromDeleteResource(), nil
}

func requestToResource(r *pb.ReportResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {
	log.Info("Report Resource Request: ", r)
	var resourceType = r.Resource.GetResourceType()
	resourceData, err := conv.ToJsonObject(r.Resource)
	if err != nil {
		return nil, err
	}

	var workspaceId, err2 = conv.ExtractWorkspaceId(r.Resource.ResourceRepresentation.Common)
	if err2 != nil {
		return nil, err2
	}

	var inventoryId, err3 = conv.ExtractInventoryId(r.Resource.InventoryId)
	if err3 != nil {
		return nil, err3
	}
	reporterType, err := conv.ExtractReporterType(r.Resource.ReporterType)
	if err != nil {
		log.Warn("Missing reporterType")
		return nil, err
	}

	reporterInstanceId, err := conv.ExtractReporterInstanceID(r.Resource.ReporterInstanceId)
	if err != nil {
		log.Warn("Missing reporterInstanceId")
		return nil, err
	}

	return conv.ResourceFromPb(resourceType, reporterType, reporterInstanceId, identity.Principal, resourceData, workspaceId, r.Resource.ResourceRepresentation, inventoryId), nil
}

func requestToDeleteResource(r *pb.DeleteResourceRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	log.Info("Delete Resource Request: ", r)

	localResourceId := r.GetLocalResourceId()
	reporterType := r.GetReporterType()

	reporterResourceId := model.ReporterResourceId{
		LocalResourceId: localResourceId,
		ReporterType:    reporterType,
		ReporterId:      identity.Principal,
		ResourceType:    identity.Type,
	}

	return reporterResourceId, nil
}

func responseFromResource() *pb.ReportResourceResponse {
	return &pb.ReportResourceResponse{}
}

func responseFromDeleteResource() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}
