//go:build sanity

package sanity

import (
	"context"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// --- Group 7: Streaming ListObjects / ListSubjects ---

func TestSanity_StreamedListObjects_ReturnsReportedResource(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "StreamedListObjects returns a resource that was just reported",
		rpc:         "ReportResource → Check → StreamedListObjects",
		given:       "A host resource reported by HBI with a workspace, tuple confirmed via Check",
		when:        "StreamedListObjects is called with the workspace as subject",
		then:        "The stream includes the reported resource ID",
	})
	client := newClient(t)
	id := uniqueID("stream-obj")
	ws := uniqueID("ws-stream-obj")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	// Wait for the tuple to propagate
	req := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, req, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	reporterType := "hbi"
	listReq := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "workspace",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   ws,
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	resourceIDs := pollStreamedListObjects(t, client, listReq, id, defaultPollTimeout)
	assert.Contains(t, resourceIDs, id, "expected reported resource in StreamedListObjects results")

	report.addDetailedStep(t.Name(), stepEntry{
		phase:  "then",
		action: "StreamedListObjects",
		input:  fmt.Sprintf("object_type=host/hbi relation=workspace subject=workspace/%s", ws),
		output: fmt.Sprintf("streamed %d objects", len(resourceIDs)),
		checkResult: fmt.Sprintf("contains %s ✓", id),
	})
}

func TestSanity_StreamedListObjects_EmptyAfterDelete(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "StreamedListObjects excludes a resource after it is deleted",
		rpc:         "ReportResource → Check → DeleteResource → Check → StreamedListObjects",
		given:       "A host resource that was reported, confirmed accessible, then deleted",
		when:        "StreamedListObjects is called after deletion is confirmed",
		then:        "The stream does not include the deleted resource ID",
	})
	client := newClient(t)
	id := uniqueID("stream-del")
	ws := uniqueID("ws-stream-del")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	checkReq := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	deleteResource(t, client, "host", id, "hbi", inst)
	assertReporterResource(t, id, "hbi", "host", 1, 0, true)

	pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)

	reporterType := "hbi"
	listReq := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "workspace",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   ws,
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	resourceIDs := collectStreamedListObjects(t, client, listReq)
	assert.NotContains(t, resourceIDs, id, "deleted resource should not appear in StreamedListObjects")

	report.addDetailedStep(t.Name(), stepEntry{
		phase:       "then",
		action:      "StreamedListObjects after delete",
		input:       fmt.Sprintf("object_type=host/hbi relation=workspace subject=workspace/%s", ws),
		output:      fmt.Sprintf("streamed %d objects", len(resourceIDs)),
		checkResult: fmt.Sprintf("does not contain %s ✓", id),
	})
}

func TestSanity_StreamedListObjects_MultipleResources(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "StreamedListObjects returns all resources in a workspace",
		rpc:         "ReportResource ×2 → Check ×2 → StreamedListObjects",
		given:       "Two host resources (A, B) reported in the same workspace",
		when:        "StreamedListObjects is called with that workspace as subject",
		then:        "The stream includes both resource IDs",
	})
	client := newClient(t)
	idA := uniqueID("stream-multi-a")
	idB := uniqueID("stream-multi-b")
	ws := uniqueID("ws-stream-multi")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, idA, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", idA, "hbi", inst) })

	reportResource(t, client, "host", "hbi", inst, idB, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", idB, "hbi", inst) })

	assertReporterResource(t, idA, "hbi", "host", 0, 0, false)
	assertReporterResource(t, idB, "hbi", "host", 0, 0, false)

	pollCheck(t, client, makeCheckReq("host", idA, "hbi", proto.String(inst), ws),
		pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
	pollCheck(t, client, makeCheckReq("host", idB, "hbi", proto.String(inst), ws),
		pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	reporterType := "hbi"
	listReq := &pb.StreamedListObjectsRequest{
		ObjectType: &pb.RepresentationType{
			ResourceType: "host",
			ReporterType: &reporterType,
		},
		Relation: "workspace",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   ws,
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}

	resourceIDs := pollStreamedListObjectsAll(t, client, listReq, []string{idA, idB}, defaultPollTimeout)
	assert.Contains(t, resourceIDs, idA, "expected host A in StreamedListObjects")
	assert.Contains(t, resourceIDs, idB, "expected host B in StreamedListObjects")

	report.addDetailedStep(t.Name(), stepEntry{
		phase:       "then",
		action:      "StreamedListObjects (multiple resources)",
		input:       fmt.Sprintf("object_type=host/hbi relation=workspace subject=workspace/%s", ws),
		output:      fmt.Sprintf("streamed %d objects", len(resourceIDs)),
		checkResult: fmt.Sprintf("contains %s and %s ✓", idA, idB),
	})
}

func TestSanity_StreamedListSubjects_ReturnsWorkspace(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "StreamedListSubjects returns the workspace that a resource belongs to",
		rpc:         "ReportResource → Check → StreamedListSubjects",
		given:       "A host resource reported with a workspace, tuple confirmed via Check",
		when:        "StreamedListSubjects is called with the host as resource and relation=workspace",
		then:        "The stream includes the workspace ID as a subject",
	})
	client := newClient(t)
	id := uniqueID("stream-subj")
	ws := uniqueID("ws-stream-subj")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	checkReq := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, checkReq, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	subjectReporterType := "rbac"
	listReq := &pb.StreamedListSubjectsRequest{
		Resource: &pb.ResourceReference{
			ResourceType: "host",
			ResourceId:   id,
			Reporter:     &pb.ReporterReference{Type: "hbi", InstanceId: proto.String(inst)},
		},
		Relation: "workspace",
		SubjectType: &pb.RepresentationType{
			ResourceType: "workspace",
			ReporterType: &subjectReporterType,
		},
	}

	subjectIDs := pollStreamedListSubjects(t, client, listReq, ws, defaultPollTimeout)
	assert.Contains(t, subjectIDs, ws, "expected workspace in StreamedListSubjects results")

	report.addDetailedStep(t.Name(), stepEntry{
		phase:       "then",
		action:      "StreamedListSubjects",
		input:       fmt.Sprintf("resource=host/%s reporter=hbi relation=workspace subject_type=workspace/rbac", id),
		output:      fmt.Sprintf("streamed %d subjects", len(subjectIDs)),
		checkResult: fmt.Sprintf("contains workspace %s ✓", ws),
	})
}

// --- Streaming helpers ---

func collectStreamedListObjects(t *testing.T, client pb.KesselInventoryServiceClient, req *pb.StreamedListObjectsRequest) []string {
	t.Helper()
	stream, err := client.StreamedListObjects(context.Background(), req)
	require.NoError(t, err, "StreamedListObjects stream creation failed")

	var ids []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "StreamedListObjects Recv failed")
		ids = append(ids, resp.GetObject().GetResourceId())
	}
	sort.Strings(ids)
	return ids
}

func collectStreamedListSubjects(t *testing.T, client pb.KesselInventoryServiceClient, req *pb.StreamedListSubjectsRequest) []string {
	t.Helper()
	stream, err := client.StreamedListSubjects(context.Background(), req)
	require.NoError(t, err, "StreamedListSubjects stream creation failed")

	var ids []string
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "StreamedListSubjects Recv failed")
		ids = append(ids, resp.GetSubject().GetResource().GetResourceId())
	}
	sort.Strings(ids)
	return ids
}

// pollStreamedListObjects retries StreamedListObjects until the target ID
// appears in the results, handling eventual consistency with SpiceDB.
func pollStreamedListObjects(t *testing.T, client pb.KesselInventoryServiceClient,
	req *pb.StreamedListObjectsRequest, targetID string, timeoutSec int,
) []string {
	t.Helper()
	for i := 0; i < timeoutSec; i++ {
		ids := collectStreamedListObjects(t, client, req)
		for _, id := range ids {
			if id == targetID {
				t.Logf("StreamedListObjects returned target %s on attempt %d", targetID, i+1)
				return ids
			}
		}
		t.Logf("StreamedListObjects attempt %d: %d results, target %s not yet present", i+1, len(ids), targetID)
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("StreamedListObjects did not return target %s within %ds", targetID, timeoutSec)
	return nil
}

// pollStreamedListObjectsAll retries until all target IDs appear.
func pollStreamedListObjectsAll(t *testing.T, client pb.KesselInventoryServiceClient,
	req *pb.StreamedListObjectsRequest, targetIDs []string, timeoutSec int,
) []string {
	t.Helper()
	for i := 0; i < timeoutSec; i++ {
		ids := collectStreamedListObjects(t, client, req)
		idSet := make(map[string]bool, len(ids))
		for _, id := range ids {
			idSet[id] = true
		}
		allPresent := true
		for _, target := range targetIDs {
			if !idSet[target] {
				allPresent = false
				break
			}
		}
		if allPresent {
			t.Logf("StreamedListObjects returned all %d targets on attempt %d", len(targetIDs), i+1)
			return ids
		}
		t.Logf("StreamedListObjects attempt %d: %d results, not all targets present yet", i+1, len(ids))
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("StreamedListObjects did not return all targets within %ds", timeoutSec)
	return nil
}

// pollStreamedListSubjects retries StreamedListSubjects until the target ID appears.
func pollStreamedListSubjects(t *testing.T, client pb.KesselInventoryServiceClient,
	req *pb.StreamedListSubjectsRequest, targetID string, timeoutSec int,
) []string {
	t.Helper()
	for i := 0; i < timeoutSec; i++ {
		ids := collectStreamedListSubjects(t, client, req)
		for _, id := range ids {
			if id == targetID {
				t.Logf("StreamedListSubjects returned target %s on attempt %d", targetID, i+1)
				return ids
			}
		}
		t.Logf("StreamedListSubjects attempt %d: %d results, target %s not yet present", i+1, len(ids), targetID)
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("StreamedListSubjects did not return target %s within %ds", targetID, timeoutSec)
	return nil
}
