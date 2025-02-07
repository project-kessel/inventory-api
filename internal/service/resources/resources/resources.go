package resourceservice

import (
	"context"
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
	//ResourceType string
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

	resourceType := r.Resource.Metadata.ResourceType
	_, isAbstract := v.AbstractResources[resourceType]

	if isAbstract {
		if r.Resource.ResourceData != nil {
			log.Errorf("Resource data is not allowed for abstract resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_type '%s' is abstract and cannot have resource_data", resourceType)
		}
	} else {
		// If the resource is not abstract, ensure `resource_data` is provided
		if r.Resource.ResourceData == nil {
			log.Errorf("Resource data is required but missing for resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_data is required for resource_type: %s", resourceType)
		}
	}

	// Extract `resource_data` if not abstract
	var resourceData map[string]interface{}
	if !isAbstract {
		resourceData = r.Resource.GetResourceData().AsMap()
		if resourceData == nil {
			log.Errorf("Resource data is empty for resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_data is invalid or empty for resource_type: %s", resourceType)
		}
	}

	reporterData := r.Resource.GetReporterData().AsMap()
	if reporterData == nil {
		log.Errorf("Reporter data is empty for resource_type: %s", resourceType)
		return nil, fmt.Errorf("reporter_data is required for resource_type: %s", resourceType)
	}
	// Create the Resource object using the parsed and converted parts
	return conv.ResourceFromJSON(r.Resource.Metadata.ResourceType, identity.Principal, resourceData, r.Resource.Metadata, reporterData), nil
}

func (c *ResourceService) resourceFromUpdateRequest(r *pb.UpdateResourceRequest, identity *authnapi.Identity) (*model.Resource, error) {

	resourceType := r.Resource.Metadata.ResourceType
	_, isAbstract := v.AbstractResources[resourceType]

	if isAbstract {
		if r.Resource.ResourceData != nil {
			log.Errorf("Resource data is not allowed for abstract resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_type '%s' is abstract and cannot have resource_data", resourceType)
		}
	} else {
		// If the resource is not abstract, ensure `resource_data` is provided
		if r.Resource.ResourceData == nil {
			log.Errorf("Resource data is required but missing for resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_data is required for resource_type: %s", resourceType)
		}
	}

	// Extract `resource_data` if not abstract
	var resourceData map[string]interface{}
	if !isAbstract {
		resourceData = r.Resource.GetResourceData().AsMap()
		if resourceData == nil {
			log.Errorf("Resource data is empty for resource_type: %s", resourceType)
			return nil, fmt.Errorf("resource_data is invalid or empty for resource_type: %s", resourceType)
		}
	}

	reporterData := r.Resource.GetReporterData().AsMap()
	if reporterData == nil {
		log.Errorf("Reporter data is empty for resource_type: %s", resourceType)
		return nil, fmt.Errorf("reporter_data is required for resource_type: %s", resourceType)
	}

	return conv.ResourceFromJSON(r.Resource.Metadata.ResourceType, identity.Principal, resourceData, r.Resource.Metadata, reporterData), nil
}

func (c *ResourceService) resourceIdFromDeleteRequest(r *pb.DeleteResourceRequest, identity *authnapi.Identity) (model.ReporterResourceId, error) {
	reporterData := r.Resource.GetReporterData().AsMap()

	if reporterData == nil {
		log.Errorf("Reporter data string is empty")
	}
	return conv.ReporterResourceIdFromJSON(r.Resource.Metadata.ResourceType, identity.Principal, reporterData), nil
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
