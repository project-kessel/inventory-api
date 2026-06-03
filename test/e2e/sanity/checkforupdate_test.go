package sanity

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// --- Group 4: CheckForUpdate combinations ---

func TestSanity_CheckForUpdate_Combinations(t *testing.T) {
	tests := []struct {
		name        string
		wsReport    string
		wsCheck     string
		deleteFirst bool
		expected    pb.Allowed
		expectToken bool
	}{
		{"matching", "ws-cfu-a", "ws-cfu-a", false, pb.Allowed_ALLOWED_TRUE, true},
		{"non_matching", "ws-cfu-a", "ws-cfu-b", false, pb.Allowed_ALLOWED_FALSE, false},
		{"after_delete", "ws-cfu-d", "ws-cfu-d", true, pb.Allowed_ALLOWED_FALSE, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newClient(t)
			id := uniqueID("cfu-" + tt.name)
			ws := uniqueID(tt.wsReport)
			wsCheck := ws
			if tt.wsReport != tt.wsCheck {
				wsCheck = uniqueID(tt.wsCheck)
			}
			inst := "inst-1"

			reportResource(t, client, "host", "hbi", inst, id, ws)

			if tt.deleteFirst {
				checkReq := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
				pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

				deleteResource(t, client, "host", id, "hbi", inst)
				assertReporterResource(t, id, "hbi", "host", 1, 0, true)

				pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
			} else {
				t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })
				assertReporterResource(t, id, "hbi", "host", 0, 0, false)
			}

			cfuReq := makeCheckForUpdateReq("host", id, "hbi", proto.String(inst), wsCheck)
			resp := pollCheckForUpdate(t, client, cfuReq, tt.expected, defaultPollTimeout)

			if tt.expectToken {
				assert.NotNil(t, resp.GetConsistencyToken(), "expected non-nil consistency token")
				assert.NotEmpty(t, resp.GetConsistencyToken().GetToken(), "expected non-empty consistency token")
			}
		})
	}
}

func TestSanity_CheckForUpdateBulk_MixedResults(t *testing.T) {
	client := newClient(t)
	id := uniqueID("cfubulk")
	wsA := uniqueID("ws-cfubulk-a")
	wsB := uniqueID("ws-cfubulk-b")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, wsA)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	// Wait for SpiceDB tuple to be created
	checkReq := makeCheckReq("host", id, "hbi", proto.String(inst), wsA)
	pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	bulkReq := &pb.CheckForUpdateBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceType: "host",
					ResourceId:   id,
					Reporter:     &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "workspace",
						ResourceId:   wsA,
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
			},
			{
				Object: &pb.ResourceReference{
					ResourceType: "host",
					ResourceId:   id,
					Reporter:     &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "workspace",
						ResourceId:   wsB,
						Reporter:     &pb.ReporterReference{Type: "rbac"},
					},
				},
			},
		},
	}

	resp, err := client.CheckForUpdateBulk(context.Background(), bulkReq)
	require.NoError(t, err, "CheckForUpdateBulk failed")
	require.Len(t, resp.GetPairs(), 2, "expected 2 pairs")

	assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.GetPairs()[0].GetItem().GetAllowed(), "pair[0] should be TRUE")
	assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.GetPairs()[1].GetItem().GetAllowed(), "pair[1] should be FALSE")

	recordBulkResult(t, "CheckForUpdateBulk (2 items)", []string{
		fmt.Sprintf("host/%s+wsA=%v", id, resp.GetPairs()[0].GetItem().GetAllowed()),
		fmt.Sprintf("host/%s+wsB=%v", id, resp.GetPairs()[1].GetItem().GetAllowed()),
	})
}
