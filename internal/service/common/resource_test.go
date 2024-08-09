package common

import (
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	biz "github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
)

func TestMetadataFromModel(t *testing.T) {
	created := time.Time{}
	updated := created.Add(500)

	data := MetadataFromModel(&biz.Metadata{
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
	})

	target := &pb.Metadata{
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

	assert.Equal(t, data, target)
}
