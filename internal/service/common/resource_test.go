package common

import (
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
)

func createPbMetadata(created time.Time, updated time.Time) pb.Metadata {
	return pb.Metadata{
		Id:              100,
		ResourceType:    "astromech",
		FirstReported:   timestamppb.New(created),
		LastReported:    timestamppb.New(updated),
		FirstReportedBy: "anakin",
		LastReportedBy:  "luke",
		Workspace:       "droids",
		Labels: []*pb.ResourceLabel{
			{
				Key:   "color",
				Value: "white",
			},
			{
				Key:   "color",
				Value: "blue",
			},
			{
				Key:   "affiliation",
				Value: "resistance",
			},
		},
	}
}

func createBizMetadata(created time.Time, updated time.Time) biz.Metadata {
	return biz.Metadata{
		ID:              100,
		CreatedAt:       created,
		UpdatedAt:       updated,
		ResourceType:    "astromech",
		FirstReportedBy: "anakin",
		LastReportedBy:  "luke",
		Workspace:       "droids",
		Reporters:       nil,
		Labels: []*biz.Label{
			{
				ID:         0,
				MetadataID: 0,
				Key:        "color",
				Value:      "white",
			},
			{
				ID:         0,
				MetadataID: 0,
				Key:        "color",
				Value:      "blue",
			},
			{
				ID:         0,
				MetadataID: 0,
				Key:        "affiliation",
				Value:      "resistance",
			},
		},
	}
}

// MetadataFromPb is used when creating a new entry
func TestMetadataFromPb(t *testing.T) {
	created := time.Time{}.Add(100)
	updated := created.Add(800)

	pbMetadata := createPbMetadata(created, updated)

	actual := MetadataFromPb(
		&pbMetadata,
		&pb.ReporterData{
			ReporterType:       pb.ReporterData_REPORTER_TYPE_HBI,
			ReporterInstanceId: "instance-001",
			FirstReported:      timestamppb.New(created),
			LastReported:       timestamppb.New(updated),
			ConsoleHref:        "console-href",
			ApiHref:            "api-href",
			LocalResourceId:    "local-01",
			ReporterVersion:    "version-123",
		},
		&authnapi.Identity{
			Tenant:     "",
			Principal:  "anakin",
			Groups:     nil,
			IsReporter: false,
			Type:       "",
			Href:       "",
			IsGuest:    false,
		},
	)

	expected := createBizMetadata(created, updated)
	expected.LastReportedBy = "anakin"
	expected.Reporters = []*biz.Reporter{
		{
			MetadataID:      0,
			ReporterID:      "anakin",
			ReporterType:    pb.ReporterData_REPORTER_TYPE_HBI.String(),
			CreatedAt:       created,
			UpdatedAt:       updated,
			LocalResourceId: "local-01",
			ReporterVersion: "version-123",
			ConsoleHref:     "console-href",
			ApiHref:         "api-href",
		},
	}

	assert.Equal(t, actual, &expected)
}

func TestMetadataFromModel(t *testing.T) {
	created := time.Time{}.Add(100)
	updated := created.Add(500)

	bizData := createBizMetadata(created, updated)
	actual := MetadataFromModel(&bizData)
	expected := createPbMetadata(created, updated)

	assert.Equal(t, actual, &expected)
}
