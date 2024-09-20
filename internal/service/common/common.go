package common

import (
	relationshipspb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	"time"

	timestamppb "google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
)

func MetadataFromPb(in *pb.Metadata, reporter *pb.ReporterData, identity *authnapi.Identity) *biz.Metadata {
	var labels []*biz.Label
	for _, t := range in.Labels {
		labels = append(labels, &biz.Label{Key: t.Key, Value: t.Value})
	}

	var updatedAt time.Time
	if in.LastReported != nil {
		updatedAt = in.LastReported.AsTime()
	} else {
		updatedAt = time.Now().UTC()
	}

	return &biz.Metadata{
		ID:              in.Id,
		ResourceType:    in.ResourceType,
		Workspace:       in.Workspace,
		CreatedAt:       in.FirstReported.AsTime(),
		UpdatedAt:       updatedAt,
		Labels:          labels,
		FirstReportedBy: identity.Principal,
		LastReportedBy:  identity.Principal,

		Reporters: []*biz.Reporter{ReporterFromPb(reporter, identity, updatedAt)},
	}
}

func ReporterFromPb(in *pb.ReporterData, identity *authnapi.Identity, defaultUpdatedAt time.Time) *biz.Reporter {
	var updatedAt time.Time
	if in.LastReported != nil {
		updatedAt = in.LastReported.AsTime()
	} else {
		updatedAt = defaultUpdatedAt
	}
	return &biz.Reporter{
		ReporterID:      identity.Principal,
		ReporterType:    in.ReporterType.String(),
		ReporterVersion: in.ReporterVersion,
		LocalResourceId: in.LocalResourceId,
		ConsoleHref:     in.ConsoleHref,
		ApiHref:         in.ApiHref,
		CreatedAt:       in.FirstReported.AsTime(),
		UpdatedAt:       updatedAt,
	}
}

func RelationshipMetadataFromPb(in *relationshipspb.Metadata, reporter *relationshipspb.ReporterData, identity *authnapi.Identity) *biz.RelationshipMetadata {
	var updatedAt time.Time
	if in.LastReported != nil {
		updatedAt = in.LastReported.AsTime()
	} else {
		updatedAt = time.Now().UTC()
	}

	return &biz.RelationshipMetadata{
		ID:               in.Id,
		RelationshipType: in.RelationshipType,
		CreatedAt:        in.FirstReported.AsTime(),
		UpdatedAt:        updatedAt,
		FirstReportedBy:  identity.Principal,
		LastReportedBy:   identity.Principal,

		Reporters: []*biz.RelationshipReporter{RelationshipReporterFromPb(reporter, identity, updatedAt)},
	}
}

func RelationshipReporterFromPb(in *relationshipspb.ReporterData, identity *authnapi.Identity, defaultUpdatedAt time.Time) *biz.RelationshipReporter {
	var updatedAt time.Time
	if in.LastReported != nil {
		updatedAt = in.LastReported.AsTime()
	} else {
		updatedAt = defaultUpdatedAt
	}
	return &biz.RelationshipReporter{
		ReporterID:             identity.Principal,
		ReporterType:           in.ReporterType.String(),
		ReporterVersion:        in.ReporterVersion,
		SubjectLocalResourceId: in.SubjectLocalResourceId,
		ObjectLocalResourceId:  in.ObjectLocalResourceId,
		CreatedAt:              in.FirstReported.AsTime(),
		UpdatedAt:              updatedAt,
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
