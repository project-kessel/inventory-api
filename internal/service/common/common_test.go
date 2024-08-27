package common

import (
	"testing"
	"time"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
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
			ReporterType:       pb.ReporterData_HBI,
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
			ReporterType:    pb.ReporterData_HBI.String(),
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

func TestReporterFromPb(t *testing.T) {
	firstReported := time.Time{}.Add(100)
	lastReported := firstReported.Add(500)

	actual := ReporterFromPb(&pb.ReporterData{
		ReporterType:       pb.ReporterData_ACM,
		ReporterInstanceId: "id-01",
		FirstReported:      timestamppb.New(firstReported),
		LastReported:       timestamppb.New(lastReported),
		ConsoleHref:        "console-href",
		ApiHref:            "api-href",
		LocalResourceId:    "local-res-01",
		ReporterVersion:    "version-123",
	}, &authnapi.Identity{
		Tenant:     "",
		Principal:  "principal-01",
		Groups:     nil,
		IsReporter: false,
		Type:       "",
		Href:       "",
		IsGuest:    false,
	}, lastReported)
	expected := biz.Reporter{
		MetadataID:      0,
		ReporterID:      "principal-01",
		ReporterType:    pb.ReporterData_ACM.String(),
		CreatedAt:       firstReported,
		UpdatedAt:       lastReported,
		LocalResourceId: "local-res-01",
		ReporterVersion: "version-123",
		ConsoleHref:     "console-href",
		ApiHref:         "api-href",
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

func TestReportersFromModel(t *testing.T) {
	created1 := time.Time{}.Add(100)
	updated1 := created1.Add(500)

	created2 := time.Time{}.Add(200)
	updated2 := created2.Add(333)

	actual := ReportersFromModel([]*biz.Reporter{
		{
			MetadataID:      100,
			ReporterID:      "reporter-01",
			ReporterType:    pb.ReporterData_ACM.String(),
			CreatedAt:       created1,
			UpdatedAt:       updated1,
			LocalResourceId: "local-resource-1",
			ReporterVersion: "reporter-version-1",
			ConsoleHref:     "console-href-1",
			ApiHref:         "api-href-1",
		},
		{
			MetadataID:      233,
			ReporterID:      "reporter-02",
			ReporterType:    pb.ReporterData_OCM.String(),
			CreatedAt:       created2,
			UpdatedAt:       updated2,
			LocalResourceId: "local-resource-2",
			ReporterVersion: "reporter-version-2",
			ConsoleHref:     "console-href-2",
			ApiHref:         "api-href-2",
		},
	})

	expected := []*pb.ReporterData{
		{
			ReporterType:       pb.ReporterData_ACM,
			ReporterInstanceId: "reporter-01",
			FirstReported:      timestamppb.New(created1),
			LastReported:       timestamppb.New(updated1),
			ConsoleHref:        "console-href-1",
			ApiHref:            "api-href-1",
			LocalResourceId:    "local-resource-1",
			ReporterVersion:    "reporter-version-1",
		},
		{
			ReporterType:       pb.ReporterData_OCM,
			ReporterInstanceId: "reporter-02",
			FirstReported:      timestamppb.New(created2),
			LastReported:       timestamppb.New(updated2),
			ConsoleHref:        "console-href-2",
			ApiHref:            "api-href-2",
			LocalResourceId:    "local-resource-2",
			ReporterVersion:    "reporter-version-2",
		},
	}

	assert.Equal(t, expected, actual)
}
