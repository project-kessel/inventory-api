package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/resources"
	"github.com/project-kessel/inventory-api/internal/biz/resources"
)

type ResourceService struct {
	pbv1beta2.UnimplementedKesselResourceServiceServer
	Ctl *resources.Usecase
}

func New(c *resources.Usecase) *ResourceService {
	return &ResourceService{
		Ctl: c,
	}
}

func (c *ResourceService) ReportResource(ctx context.Context, r *pbv1beta2.ReportResourceRequest) (*pbv1beta2.ReportResourceResponse, error) {
	log.Info("I am in the new Resource Service Report method!", ctx, r)
	return nil, nil
}

func (c *ResourceService) DeleteResource(ctx context.Context, r *pbv1beta2.DeleteResourceRequest) (*pbv1beta2.DeleteResourceResponse, error) {
	log.Info("I am in the new Resource Service Delete method!", ctx, r)
	return nil, nil
}
