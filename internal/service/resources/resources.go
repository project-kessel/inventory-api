package resources

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/resources"

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
	log.Info("I am in the new Resource Service Report method!", ctx, r)
	return nil, nil
}

func (c *ResourceService) DeleteK8SCluster(ctx context.Context, r *pb.DeleteResourceRequest) (*pb.DeleteResourceResponse, error) {
	log.Info("I am in the new Resource Service Delete method!", ctx, r)
	return nil, nil
}
