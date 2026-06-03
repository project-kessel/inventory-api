package sanity

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// --- Group 6: Lifecycle / Churn ---

func TestSanity_ReportDeleteReReport_Revive(t *testing.T) {
	client := newClient(t)
	id := uniqueID("revive-host")
	ws := uniqueID("ws-revive")
	inst := "inst-1"

	// Step 1: Report -> TRUE
	reportResource(t, client, "host", "hbi", inst, id, ws)
	assertReporterResource(t, id, "hbi", "host", 0, 0, false)
	res := findResourceByReporterKey(t, id, "hbi", "host")
	assertCommonRepresentation(t, res.ID, 0, ws)

	req := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, req, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	// Step 2: Delete -> FALSE
	deleteResource(t, client, "host", id, "hbi", inst)
	assertReporterResource(t, id, "hbi", "host", 1, 0, true)
	rr := findReporterResource(t, id, "hbi", "host")
	assertReporterRepresentation(t, rr.ID, 1, 0, true)

	pollCheck(t, client, req, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)

	// Step 3: Re-report (revive) -> TRUE
	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	// After revive: generation=1, rep_version=0, tombstone=false
	assertReporterResource(t, id, "hbi", "host", 0, 1, false)
	rr = findReporterResource(t, id, "hbi", "host")
	assertReporterRepresentation(t, rr.ID, 0, 1, false)
	res = findResourceByReporterKey(t, id, "hbi", "host")
	assertCommonRepresentation(t, res.ID, 1, ws)

	pollCheck(t, client, req, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
}

func TestSanity_MultiResourceChurn(t *testing.T) {
	client := newClient(t)
	idA := uniqueID("churn-a")
	idB := uniqueID("churn-b")
	idC := uniqueID("churn-c")
	ws := uniqueID("ws-churn")
	inst := "inst-1"

	// Step 1: Report 3 hosts
	reportResource(t, client, "host", "hbi", inst, idA, ws)
	reportResource(t, client, "host", "hbi", inst, idB, ws)
	reportResource(t, client, "host", "hbi", inst, idC, ws)

	for _, id := range []string{idA, idB, idC} {
		assertReporterResource(t, id, "hbi", "host", 0, 0, false)
	}

	reqA := makeCheckReq("host", idA, "hbi", proto.String(inst), ws)
	reqB := makeCheckReq("host", idB, "hbi", proto.String(inst), ws)
	reqC := makeCheckReq("host", idC, "hbi", proto.String(inst), ws)

	pollCheck(t, client, reqA, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
	pollCheck(t, client, reqB, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
	pollCheck(t, client, reqC, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	// Step 2: Delete host-b
	deleteResource(t, client, "host", idB, "hbi", inst)

	assertReporterResource(t, idA, "hbi", "host", 0, 0, false) // unchanged
	assertReporterResource(t, idB, "hbi", "host", 1, 0, true)  // tombstoned
	assertReporterResource(t, idC, "hbi", "host", 0, 0, false) // unchanged

	pollCheck(t, client, reqB, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
	// A and C should still be TRUE (no change needed, already confirmed)

	// Step 3: Delete host-a and host-c
	deleteResource(t, client, "host", idA, "hbi", inst)
	deleteResource(t, client, "host", idC, "hbi", inst)

	for _, id := range []string{idA, idB, idC} {
		assertReporterResource(t, id, "hbi", "host", 1, 0, true)
	}

	pollCheck(t, client, reqA, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
	pollCheck(t, client, reqC, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
}
