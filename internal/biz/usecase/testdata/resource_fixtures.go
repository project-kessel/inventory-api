package testdata

import (
	"time"

	"github.com/google/uuid"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model/v1beta2"
	"google.golang.org/protobuf/types/known/structpb"
)

// ReportResourceRequestBothRepresentations creates a ReportResourceRequestBothRepresentations based on real host data
func ReportResourceRequestBothRepresentations() *pb.ReportResourceRequest {
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

// ReportResourceRequestReporterOnly creates a ReportResourceRequestBothRepresentations with only reporter representation
func ReportResourceRequestReporterOnly() *pb.ReportResourceRequest {
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
			Common:   nil, // No common representation
			Reporter: reporter,
		},
	}
}

// ExpectedResource creates the expected Resource struct for resource fixtures
func ExpectedResource(resourceID uuid.UUID) *v1beta2.Resource {
	return &v1beta2.Resource{
		ID:               resourceID,
		Type:             "host",
		ConsistencyToken: "", // Should be empty (ktn is empty)
	}
}

// ExpectedReporterRepresentation creates the expected ReporterRepresentation struct for resource fixtures
func ExpectedReporterRepresentation(data model.JsonObject, createdAt, updatedAt time.Time) *v1beta2.ReporterRepresentation {
	commonVersion := 1 // Business logic sets this to 1 when common representation exists
	return &v1beta2.ReporterRepresentation{
		BaseRepresentation: v1beta2.BaseRepresentation{
			Data:      data,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		LocalResourceID:    "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		ReporterType:       "hbi",
		ResourceType:       "host",
		Version:            1,
		ReporterInstanceID: "3088be62-1c60-4884-b133-9200542d0b3f",
		Generation:         1,
		APIHref:            "https://apiHref.com/",
		ConsoleHref:        "https://www.console.com/",
		CommonVersion:      &commonVersion, // Set to 1 when common representation exists
		Tombstone:          false,
		ReporterVersion:    "2.7.16",
	}
}

// ExpectedReporterRepresentationReporterOnly creates the expected ReporterRepresentation struct for reporter-only scenarios
func ExpectedReporterRepresentationReporterOnly(data model.JsonObject, createdAt, updatedAt time.Time) *v1beta2.ReporterRepresentation {
	commonVersion := 1 // Business logic still sets this even for reporter-only scenarios
	return &v1beta2.ReporterRepresentation{
		BaseRepresentation: v1beta2.BaseRepresentation{
			Data:      data,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		LocalResourceID:    "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		ReporterType:       "hbi",
		ResourceType:       "host",
		Version:            1,
		ReporterInstanceID: "3088be62-1c60-4884-b133-9200542d0b3f",
		Generation:         1,
		APIHref:            "https://apiHref.com/",
		ConsoleHref:        "https://www.console.com/",
		CommonVersion:      &commonVersion, // Still set to 1 even without common representation
		Tombstone:          false,
		ReporterVersion:    "2.7.16",
	}
}

// ExpectedCommonRepresentation creates the expected CommonRepresentation struct for resource fixtures
func ExpectedCommonRepresentation(data model.JsonObject, createdAt, updatedAt time.Time, localResourceID, reportedBy string) *v1beta2.CommonRepresentation {
	return &v1beta2.CommonRepresentation{
		BaseRepresentation: v1beta2.BaseRepresentation{
			Data:      data,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
		LocalResourceID: localResourceID, // This is dynamically generated
		ReporterType:    "inventory",
		ResourceType:    "host",
		Version:         1,
		ReportedBy:      reportedBy, // This includes the reporter instance ID
	}
}

// ExpectedCommonRepresentationReference creates the expected RepresentationReference struct for inventory (common) representation
func ExpectedCommonRepresentationReference(resourceID uuid.UUID, localResourceID string) *v1beta2.RepresentationReference {
	return &v1beta2.RepresentationReference{
		ResourceID:            resourceID,
		LocalResourceID:       localResourceID, // This is dynamically generated
		ReporterType:          "inventory",
		ResourceType:          "host",
		ReporterInstanceID:    "", // Empty for inventory references
		RepresentationVersion: 1,  // Latest version of common representation
		Generation:            1,
		Tombstone:             false,
	}
}

// ExpectedReporterRepresentationReference creates the expected RepresentationReference struct for reporter representation
func ExpectedReporterRepresentationReference(resourceID uuid.UUID) *v1beta2.RepresentationReference {
	return &v1beta2.RepresentationReference{
		ResourceID:            resourceID,
		LocalResourceID:       "dd1b73b9-3e33-4264-968c-e3ce55b9afec",
		ReporterType:          "hbi",
		ResourceType:          "host",
		ReporterInstanceID:    "3088be62-1c60-4884-b133-9200542d0b3f",
		RepresentationVersion: 1, // Latest version of reporter representation
		Generation:            1,
		Tombstone:             false,
	}
}
