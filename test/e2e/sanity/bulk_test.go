//go:build sanity

package sanity

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// --- Group 5: CheckBulk + CheckSelf ---

func TestSanity_CheckBulk_MultipleHosts(t *testing.T) {
	client := newClient(t)
	hostA := uniqueID("bulk-host-a")
	hostB := uniqueID("bulk-host-b")
	wsA := uniqueID("ws-bulk-a")
	wsB := uniqueID("ws-bulk-b")
	wsWrong := uniqueID("ws-bulk-wrong")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, hostA, wsA)
	t.Cleanup(func() { deleteResource(t, client, "host", hostA, "hbi", inst) })

	reportResource(t, client, "host", "hbi", inst, hostB, wsB)
	t.Cleanup(func() { deleteResource(t, client, "host", hostB, "hbi", inst) })

	// DB assertions
	assertReporterResource(t, hostA, "hbi", "host", 0, 0, false)
	assertReporterResource(t, hostB, "hbi", "host", 0, 0, false)

	// Wait for both tuples
	pollCheck(t, client, makeCheckReq("host", hostA, "hbi", proto.String(inst), wsA),
		pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
	pollCheck(t, client, makeCheckReq("host", hostB, "hbi", proto.String(inst), wsB),
		pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	bulkReq := &pb.CheckBulkRequest{
		Items: []*pb.CheckBulkRequestItem{
			{ // hostA + wsA -> TRUE
				Object: &pb.ResourceReference{
					ResourceType: "host", ResourceId: hostA,
					Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "workspace", ResourceId: wsA,
						Reporter: &pb.ReporterReference{Type: "rbac"},
					},
				},
			},
			{ // hostB + wsB -> TRUE
				Object: &pb.ResourceReference{
					ResourceType: "host", ResourceId: hostB,
					Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "workspace", ResourceId: wsB,
						Reporter: &pb.ReporterReference{Type: "rbac"},
					},
				},
			},
			{ // hostA + wsWrong -> FALSE
				Object: &pb.ResourceReference{
					ResourceType: "host", ResourceId: hostA,
					Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
				Subject: &pb.SubjectReference{
					Resource: &pb.ResourceReference{
						ResourceType: "workspace", ResourceId: wsWrong,
						Reporter: &pb.ReporterReference{Type: "rbac"},
					},
				},
			},
		},
	}

	// Retry CheckBulk to handle eventual consistency across SpiceDB nodes
	var resp *pb.CheckBulkResponse
	for attempt := 1; attempt <= defaultPollTimeout; attempt++ {
		var err error
		resp, err = client.CheckBulk(context.Background(), bulkReq)
		require.NoError(t, err, "CheckBulk failed")
		require.Len(t, resp.GetPairs(), 3, "expected 3 pairs")

		if resp.GetPairs()[0].GetItem().GetAllowed() == pb.Allowed_ALLOWED_TRUE &&
			resp.GetPairs()[1].GetItem().GetAllowed() == pb.Allowed_ALLOWED_TRUE &&
			resp.GetPairs()[2].GetItem().GetAllowed() == pb.Allowed_ALLOWED_FALSE {
			t.Logf("CheckBulk returned expected results on attempt %d", attempt)
			break
		}
		if attempt == defaultPollTimeout {
			t.Fatalf("CheckBulk did not converge within %d attempts", defaultPollTimeout)
		}
		t.Logf("CheckBulk attempt %d: results not yet consistent, retrying...", attempt)
		time.Sleep(1 * time.Second)
	}

	assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.GetPairs()[0].GetItem().GetAllowed(), "pair[0] hostA+wsA should be TRUE")
	assert.Equal(t, pb.Allowed_ALLOWED_TRUE, resp.GetPairs()[1].GetItem().GetAllowed(), "pair[1] hostB+wsB should be TRUE")
	assert.Equal(t, pb.Allowed_ALLOWED_FALSE, resp.GetPairs()[2].GetItem().GetAllowed(), "pair[2] hostA+wsWrong should be FALSE")

	recordBulkResult(t, "CheckBulk (3 items)", []string{
		fmt.Sprintf("host/%s+wsA=%v", hostA, resp.GetPairs()[0].GetItem().GetAllowed()),
		fmt.Sprintf("host/%s+wsB=%v", hostB, resp.GetPairs()[1].GetItem().GetAllowed()),
		fmt.Sprintf("host/%s+wsWrong=%v", hostA, resp.GetPairs()[2].GetItem().GetAllowed()),
	})
}

func TestSanity_CheckSelf_ReturnsError(t *testing.T) {
	client := newClient(t)
	id := uniqueID("self-host")
	ws := uniqueID("ws-self")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	req := &pb.CheckSelfRequest{
		Object: &pb.ResourceReference{
			ResourceType: "host", ResourceId: id,
			Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
		},
		Relation: "workspace",
	}

	_, err := client.CheckSelf(context.Background(), req)
	require.Error(t, err, "CheckSelf should fail in unauthenticated mode")
	t.Logf("CheckSelf error (expected): %v", err)
	recordError(t, "CheckSelf (expected error)", err)
}

func TestSanity_CheckSelfBulk_ReturnsError(t *testing.T) {
	client := newClient(t)
	id := uniqueID("selfbulk-host")
	ws := uniqueID("ws-selfbulk")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	req := &pb.CheckSelfBulkRequest{
		Items: []*pb.CheckSelfBulkRequestItem{
			{
				Object: &pb.ResourceReference{
					ResourceType: "host", ResourceId: id,
					Reporter: &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
				},
				Relation: "workspace",
			},
		},
	}

	_, err := client.CheckSelfBulk(context.Background(), req)
	require.Error(t, err, "CheckSelfBulk should fail in unauthenticated mode")
	t.Logf("CheckSelfBulk error (expected): %v", err)
	recordError(t, "CheckSelfBulk (expected error)", err)
}
