package common

import (
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
)

func MetadataFromPb(in *pb.Metadata, reporter *pb.ReporterData, identity *authnapi.Identity, resourceType string) *biz.Metadata {
	var labels []*biz.Label
	for _, t := range in.Labels {
		labels = append(labels, &biz.Label{Key: t.Key, Value: t.Value})
	}

	return &biz.Metadata{
		ID:              in.Id,
		ResourceType:    resourceType,
		Workspace:       in.Workspace,
		CreatedAt:       in.FirstReported.AsTime(),
		UpdatedAt:       in.LastReported.AsTime(),
		Labels:          labels,
		FirstReportedBy: identity.Principal,
		LastReportedBy:  identity.Principal,

		Reporters: []*biz.Reporter{ReporterFromPb(reporter, identity)},
	}
}

func ReporterFromPb(in *pb.ReporterData, identity *authnapi.Identity) *biz.Reporter {
	return &biz.Reporter{
		ReporterID:      identity.Principal,
		ReporterType:    in.ReporterType.String(),
		ReporterVersion: in.ReporterVersion,
		LocalResourceId: in.LocalResourceId,
		ConsoleHref:     in.ConsoleHref,
		ApiHref:         in.ApiHref,
		CreatedAt:       in.FirstReported.AsTime(),
		UpdatedAt:       in.LastReported.AsTime(),
	}
}

func MetadataFromModel(in *biz.Metadata) *pb.Metadata {
	var labels []*pb.ResourceLabel
	for _, t := range in.Labels {
		labels = append(labels, &pb.ResourceLabel{Key: t.Key, Value: t.Value})
	}

	return &pb.Metadata{
		Id:              in.ID,
		ResourceType:    in.ResourceType,
		FirstReported:   timestamppb.New(in.CreatedAt),
		LastReported:    timestamppb.New(in.UpdatedAt),
		FirstReportedBy: in.FirstReportedBy,
		LastReportedBy:  in.LastReportedBy,
		Labels:          labels,
		Workspace:       in.Workspace,
	}
}

func ReportersFromModel(in []*biz.Reporter) []*pb.ReporterData {
	var reporters []*pb.ReporterData
	for _, r := range in {
		reporters = append(reporters, &pb.ReporterData{
			ReporterInstanceId: r.ReporterID,
			ReporterType:       pb.ReporterData_ReporterType(pb.ReporterData_ReporterType_value[r.ReporterType]),
			ReporterVersion:    r.ReporterVersion,

			LocalResourceId: r.LocalResourceId,

			FirstReported: timestamppb.New(r.CreatedAt),
			LastReported:  timestamppb.New(r.UpdatedAt),

			ConsoleHref: r.ConsoleHref,
			ApiHref:     r.ApiHref,
		})
	}
	return reporters
}
