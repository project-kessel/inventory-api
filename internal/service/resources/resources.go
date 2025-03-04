package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	resources2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/resources"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
)

type ResourceService struct {
	resources2.UnimplementedKesselResourceServiceServer
	Ctl *resources.Usecase
}

func New(c *resources.Usecase) *ResourceService {
	return &ResourceService{
		Ctl: c,
	}
}

func (c *ResourceService) ReportResource(ctx context.Context, r *resources2.ReportResourceRequest) (*resources2.ReportResourceResponse, error) {
	log.Info("I am in the new Resource Service Report method!", ctx, r)
	return nil, nil
}

func (c *ResourceService) DeleteK8SCluster(ctx context.Context, r *resources2.DeleteResourceRequest) (*resources2.DeleteResourceResponse, error) {
	log.Info("I am in the new Resource Service Delete method!", ctx, r)
	return nil, nil
}
