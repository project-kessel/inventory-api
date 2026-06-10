//go:build sanity

package sanity

import (
	"testing"

	"google.golang.org/protobuf/proto"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// --- Group 1: Report + Check (Access Exists) ---

func TestSanity_ReportHost_CheckAllowed(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "Reporting a resource creates a workspace tuple that grants access",
		rpc:         "ReportResource → Check",
		given:       "A host resource reported by HBI with a workspace",
		when:        "Check is called with the correct workspace as subject",
		then:        "Check returns ALLOWED_TRUE and DB shows ver=0, gen=0, tombstone=false",
	})
	client := newClient(t)
	id := uniqueID("sanity-host")
	ws := uniqueID("ws")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	// DB assertions after report
	assertReporterResource(t, id, "hbi", "host", 0, 0, false)
	rr := findReporterResource(t, id, "hbi", "host")
	assertReporterRepresentation(t, rr.ID, 0, 0, false)
	res := findResourceByReporterKey(t, id, "hbi", "host")
	assertCommonRepresentation(t, res.ID, 0, ws)

	// Check workspace -> TRUE
	req := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, req, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
}

func TestSanity_Check_WrongWorkspace(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "Check denies access when the workspace does not match",
		rpc:         "ReportResource → Check",
		given:       "A host resource reported with workspace-A",
		when:        "Check is called with workspace-B as subject",
		then:        "Check returns ALLOWED_FALSE",
	})
	client := newClient(t)
	id := uniqueID("sanity-host-wrong")
	wsRight := uniqueID("ws-right")
	wsWrong := uniqueID("ws-wrong")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, wsRight)
	t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

	assertReporterResource(t, id, "hbi", "host", 0, 0, false)
	res := findResourceByReporterKey(t, id, "hbi", "host")
	assertCommonRepresentation(t, res.ID, 0, wsRight)

	// Check with wrong workspace -> FALSE (no tuple exists for wsWrong)
	req := makeCheckReq("host", id, "hbi", proto.String(inst), wsWrong)
	pollCheck(t, client, req, pb.Allowed_ALLOWED_FALSE, 10)
}

// --- Group 2: Delete + Check (Access Lost) ---

func TestSanity_DeleteHost_AccessLost(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "Deleting a resource removes the workspace tuple and revokes access",
		rpc:         "ReportResource → Check → DeleteResource → Check",
		given:       "A host resource with confirmed ALLOWED_TRUE access",
		when:        "The resource is deleted",
		then:        "Check returns ALLOWED_FALSE and DB shows tombstone=true",
	})
	client := newClient(t)
	id := uniqueID("sanity-del-host")
	ws := uniqueID("ws")
	inst := "inst-1"

	reportResource(t, client, "host", "hbi", inst, id, ws)
	assertReporterResource(t, id, "hbi", "host", 0, 0, false)

	// Confirm access exists
	req := makeCheckReq("host", id, "hbi", proto.String(inst), ws)
	pollCheck(t, client, req, pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)

	// Delete
	deleteResource(t, client, "host", id, "hbi", inst)

	// DB assertions after delete
	assertReporterResource(t, id, "hbi", "host", 1, 0, true)
	rr := findReporterResource(t, id, "hbi", "host")
	assertReporterRepresentation(t, rr.ID, 1, 0, true)

	// Confirm access lost
	pollCheck(t, client, req, pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
}

// --- Group 3: Check Combinations (table-driven) ---

func TestSanity_Check_Combinations(t *testing.T) {
	report.describe(t.Name(), testSpec{
		description: "Check returns correct results across workspace and instance_id combinations",
		rpc:         "ReportResource → Check",
		given:       "A host resource reported with a specific workspace",
		when:        "Check is called with matching/non-matching workspace, with/without instance_id",
		then:        "Each combination returns the expected ALLOWED_TRUE or ALLOWED_FALSE",
	})
	tests := []struct {
		name        string
		wsReport    string
		wsCheck     string
		includeInst bool
		expected    pb.Allowed
	}{
		{"matching_workspace", "ws-a", "ws-a", true, pb.Allowed_ALLOWED_TRUE},
		{"non_matching_workspace", "ws-a", "ws-b", true, pb.Allowed_ALLOWED_FALSE},
		{"no_instance_in_check_ref", "ws-f", "ws-f", false, pb.Allowed_ALLOWED_TRUE},
		{"with_instance_in_check_ref", "ws-g", "ws-g", true, pb.Allowed_ALLOWED_TRUE},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newClient(t)
			id := uniqueID("combo-" + tt.name)
			ws := uniqueID(tt.wsReport)
			wsCheck := ws
			if tt.wsReport != tt.wsCheck {
				wsCheck = uniqueID(tt.wsCheck)
			}
			inst := "inst-1"

			reportResource(t, client, "host", "hbi", inst, id, ws)
			t.Cleanup(func() { deleteResource(t, client, "host", id, "hbi", inst) })

			// DB assertions
			assertReporterResource(t, id, "hbi", "host", 0, 0, false)
			res := findResourceByReporterKey(t, id, "hbi", "host")
			assertCommonRepresentation(t, res.ID, 0, ws)

			var instPtr *string
			if tt.includeInst {
				instPtr = proto.String(inst)
			}
			req := makeCheckReq("host", id, "hbi", instPtr, wsCheck)

			timeout := defaultPollTimeout
			if tt.expected == pb.Allowed_ALLOWED_FALSE {
				timeout = 10
			}
			pollCheck(t, client, req, tt.expected, timeout)
		})
	}
}
