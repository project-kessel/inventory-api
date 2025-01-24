package resourceservice

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
	"github.com/project-kessel/inventory-api/internal/middleware"
	v "github.com/project-kessel/inventory-api/internal/middleware"
	conv "github.com/project-kessel/inventory-api/internal/service/common"
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

func (c *ResourceService) CreateResource(ctx context.Context, r *pb.CreateResourceRequest) (*pb.CreateResourceResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if !isValidResourceType(r.Resource.Metadata.ResourceType) {
		return nil, fmt.Errorf("invalid resource_type: %s", r.Resource.Metadata.ResourceType)
	}

	if resource, err := c.resourceFromCreateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.Create(ctx, resource); err == nil {
			return createResponseFromResource(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *ResourceService) UpdateResource(ctx context.Context, r *pb.UpdateResourceRequest) (*pb.UpdateResourceResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if !isValidResourceType(r.Resource.Metadata.ResourceType) {
		return nil, fmt.Errorf("invalid resource_type: %s", r.Resource.Metadata.ResourceType)
	}

	if resource, err := c.resourceFromUpdateRequest(r, identity); err == nil {
		if resp, err := c.Ctl.Update(ctx, resource, model.ReporterResourceIdFromResource(resource)); err == nil {
			return updateResponseFromResource(resp), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *ResourceService) DeleteResource(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return nil, err
	}

	if !isValidResourceType(r.Resource.Metadata.ResourceType) {
		return nil, fmt.Errorf("invalid resource_type: %s", r.Resource.Metadata.ResourceType)
	}

	if resourceId, err := c.resourceIdFromDeleteRequest(r, identity); err == nil {
		if err := c.Ctl.Delete(ctx, resourceId); err == nil {
			return toDeleteResponse(), nil
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
}

func (c *ResourceService) resourceFromCreateRequest(r *pb.CreateResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {

	if r.Resource.ResourceData == nil {
		log.Errorf("Resource data empty")
	}

	// Extract the `resource_data` field as a string
	resourceDataStr := r.Resource.ResourceData.GetResourceData()

	// Check if the extracted data is empty
	if resourceDataStr == "" {
		log.Errorf("Resource data string is empty")
	}

	// Parse the JSON string into a map
	var resourceData map[string]interface{}
	if err := json.Unmarshal([]byte(resourceDataStr), &resourceData); err != nil {
		log.Errorf("Failed to unmarshall json")
	}

	// Create the Resource object using the parsed and converted parts
	return conv.ResourceFromPb(r.Resource.Metadata.ResourceType, identity.Principal, resourceData, r.Resource.Metadata, r.Resource.ReporterData), nil
}

func (c *ResourceService) resourceFromUpdateRequest(r *pb.UpdateResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {

	if r.Resource.ResourceData == nil {
		log.Errorf("Resource data empty")
	}

	// Extract the `resource_data` field as a string
	resourceDataStr := r.Resource.ResourceData.GetResourceData()

	// Check if the extracted data is empty
	if resourceDataStr == "" {
		log.Errorf("Resource data string is empty")
	}

	// Parse the JSON string into a map
	var resourceData map[string]interface{}
	if err := json.Unmarshal([]byte(resourceDataStr), &resourceData); err != nil {
		log.Errorf("Failed to unmarshall json")
	}

	return conv.ResourceFromPb(r.Resource.Metadata.ResourceType, identity.Principal, resourceData, r.Resource.Metadata, r.Resource.ReporterData), nil
}

func (c *ResourceService) resourceIdFromDeleteRequest(r *pb.DeleteResourceRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	return conv.ReporterResourceIdFromPb(r.Resource.Metadata.ResourceType, identity.Principal, r.Resource.ReporterData), nil
}

func createResponseFromResource(c *model.Resource) *pb.CreateResourceResponse {
	return &pb.CreateResourceResponse{}
}

func updateResponseFromResource(c *model.Resource) *pb.UpdateResourceResponse {
	return &pb.UpdateResourceResponse{}
}

func toDeleteResponse() *pb.DeleteResourceResponse {
	return &pb.DeleteResourceResponse{}
}

func isValidResourceType(resourceType string) bool {
	_, exists := v.AllowedResourceTypes[resourceType]
	return exists
}
