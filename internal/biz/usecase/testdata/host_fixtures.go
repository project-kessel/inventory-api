package testdata

import (
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"google.golang.org/protobuf/types/known/structpb"
)

// HostReportResourceRequest creates a ReportResourceRequest based on real host data
func HostReportResourceRequest() *pb.ReportResourceRequest {
	// Common representation
	commonFields := map[string]*structpb.Value{
		"workspace_id": structpb.NewStringValue("a64d17d0-aec3-410a-acd0-e0b85b22c076"),
	}
	common := &structpb.Struct{Fields: commonFields}

	// Reporter representation
	reporterFields := map[string]*structpb.Value{
		"satellite_id":            structpb.NewStringValue("2c4196f1-0371-4f4c-8913-e113cfaa6e67"),
		"subscription_manager_id": structpb.NewStringValue("af94f92b-0b65-4cac-b449-6b77e665a08f"),
		"insights_inventory_id":   structpb.NewStringValue("05707922-7b0a-4fe6-982d-6adbc7695b8f"),
		"ansible_host":            structpb.NewStringValue("host-1"),
	}
	reporter := &structpb.Struct{Fields: reporterFields}

	// Metadata
	metadata := &pb.RepresentationMetadata{
		LocalResourceId: "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		ApiHref:         "https://apiHref.com/",
		ConsoleHref:     &[]string{"https://www.console.com/"}[0],
		ReporterVersion: &[]string{"2.7.16"}[0],
	}

	return &pb.ReportResourceRequest{
		Type:               "host",
		ReporterType:       "hbi",
		ReporterInstanceId: "3088be62-1c60-4884-b133-9200542d0b3f",
		Representations: &pb.ResourceRepresentations{
			Metadata: metadata,
			Common:   common,
			Reporter: reporter,
		},
	}
}
