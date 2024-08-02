package common

import (
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	models "github.com/project-kessel/inventory-api/internal/biz/common"
)

func MetadataFromPb(in *pb.Metadata, identity *authnapi.Identity) *models.Metadata {
	var tags []*models.Tag
	for _, t := range in.Tags {
		tags = append(tags, &models.Tag{Key: t.Key, Value: t.Value})
	}

	return &models.Metadata{
		ID:           in.Id,
		ResourceType: in.ResourceType,
		Workspace:    in.Workspace,
		Tags:         tags,

		FirstReportedBy: identity.Principal,
		LastReportedBy:  identity.Principal,

		FirstReported: time.Now(),
		LastReported:  time.Now(),
	}
}

func ReporterFromPb(in *pb.ReporterData, identity *authnapi.Identity) *models.Reporter {
	return &models.Reporter{
		ReporterID:      identity.Principal,
		ReporterType:    in.ReporterType.String(),
		ReporterVersion: in.ReporterVersion,

		LocalResourceId: in.LocalResourceId,

		ConsoleHref: in.ConsoleHref,
		ApiHref:     in.ApiHref,
	}
}

func MetadataFromModel(in *models.Metadata) *pb.Metadata {
	var tags []*pb.ResourceTag
	for _, t := range in.Tags {
		tags = append(tags, &pb.ResourceTag{Key: t.Key, Value: t.Value})
	}

	return &pb.Metadata{
		Id:              in.ID,
		ResourceType:    in.ResourceType,
		FirstReported:   timestamppb.New(in.FirstReported),
		LastReported:    timestamppb.New(in.LastReported),
		FirstReportedBy: in.FirstReportedBy,
		LastReportedBy:  in.LastReportedBy,
		Tags:            tags,
	}
}

func ReportersFromModel(in []*models.Reporter) []*pb.ReporterData {
	var reporters []*pb.ReporterData
	for _, r := range in {
		reporters = append(reporters, &pb.ReporterData{
			ReporterInstanceId: r.ReporterID,
			ReporterType:       pb.ReporterData_ReporterTypeEnum(pb.ReporterData_ReporterTypeEnum_value[r.ReporterType]),
			ReporterVersion:    r.ReporterVersion,

			LocalResourceId: r.LocalResourceId,

			ConsoleHref: r.ConsoleHref,
			ApiHref:     r.ApiHref,
		})
	}
	return reporters
}
