//go:build sanity

package sanity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

var (
	grpcURL string
	db      *gorm.DB
)

// --- Test report infrastructure ---

type stepEntry struct {
	action      string
	dbState     string
	checkResult string
}

type testReportCollector struct {
	mu      sync.Mutex
	order   []string
	steps   map[string][]stepEntry
	results map[string]string
	count   int
}

var report = &testReportCollector{
	steps:   make(map[string][]stepEntry),
	results: make(map[string]string),
}

func (r *testReportCollector) startTest(testName string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.count++
	n := r.count
	sep := strings.Repeat("─", 80)
	_, _ = fmt.Fprintf(os.Stdout, "\n%s%s%s\n", colorDim, sep, colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "%s[%d] %s%s\n", colorBold, n, testName, colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s\n", colorDim, sep, colorReset)
	return n
}

func (r *testReportCollector) addStep(testName, action, dbState, checkResult string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.steps[testName]; !exists {
		r.order = append(r.order, testName)
	}
	r.steps[testName] = append(r.steps[testName], stepEntry{
		action:      action,
		dbState:     dbState,
		checkResult: checkResult,
	})
}

func (r *testReportCollector) setResult(testName, result string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results[testName] = result
}

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func printReport() {
	report.mu.Lock()
	defer report.mu.Unlock()

	separator := strings.Repeat("═", 80)
	_, _ = fmt.Fprintf(os.Stdout, "\n%s%s%s\n", colorBold, separator, colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "%s  SANITY TEST REPORT%s\n", colorBold, colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s\n\n", colorBold, separator, colorReset)

	passed, failed, total := 0, 0, 0
	for _, name := range report.order {
		total++
		result := report.results[name]
		var resultColor string
		switch result {
		case "PASS":
			resultColor = colorGreen
			passed++
		case "FAIL":
			resultColor = colorRed
			failed++
		default:
			result = "?"
			resultColor = colorYellow
		}

		_, _ = fmt.Fprintf(os.Stdout, "%s▸ [%d] %s%s  %s[%s]%s\n",
			colorBold, total, name, colorReset, resultColor, result, colorReset)

		steps := report.steps[name]
		for i, s := range steps {
			stepNum := fmt.Sprintf("%d", i+1)

			_, _ = fmt.Fprintf(os.Stdout, "  %s%s.%s %s\n",
				colorCyan, stepNum, colorReset, s.action)

			if s.dbState != "" {
				_, _ = fmt.Fprintf(os.Stdout, "     %sDB: %s%s\n",
					colorDim, s.dbState, colorReset)
			}
			if s.checkResult != "" {
				resultColor := colorGreen
				if strings.Contains(s.checkResult, "FALSE") || strings.Contains(s.checkResult, "TIMEOUT") || strings.Contains(s.checkResult, "error") {
					resultColor = colorYellow
				}
				_, _ = fmt.Fprintf(os.Stdout, "     %s→ %s%s\n",
					resultColor, s.checkResult, colorReset)
			}
		}
		_, _ = fmt.Fprintln(os.Stdout)
	}

	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s\n", colorBold, strings.Repeat("─", 80), colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "  Total: %d  |  %sPassed: %d%s  |  %sFailed: %d%s\n",
		total, colorGreen, passed, colorReset, colorRed, failed, colorReset)
	_, _ = fmt.Fprintf(os.Stdout, "%s%s%s\n\n", colorBold, strings.Repeat("─", 80), colorReset)
}

// --- TestMain ---

func TestMain(m *testing.M) {
	grpcURL = envOr("INV_GRPC_URL", "localhost:9000")

	dsn := fmt.Sprintf("host=%s user=%s password=%s port=%s dbname=%s sslmode=disable",
		envOr("POSTGRES_HOST", "localhost"),
		envOr("POSTGRES_USER", "postgres"),
		envOr("POSTGRES_PASSWORD", ""),
		envOr("POSTGRES_PORT", "5432"),
		envOr("POSTGRES_DB", "kessel-inventory"),
	)
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()
	printReport()
	os.Exit(code)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// --- gRPC helpers ---

type bearerAuth struct {
	token string
}

func (b *bearerAuth) GetRequestMetadata(_ context.Context, _ ...string) (map[string]string, error) {
	return map[string]string{"authorization": fmt.Sprintf("Bearer %s", b.token)}, nil
}

func (b *bearerAuth) RequireTransportSecurity() bool { return false }

func newClient(t *testing.T) pb.KesselInventoryServiceClient {
	t.Helper()
	report.startTest(t.Name())
	conn, err := grpc.NewClient(
		grpcURL,
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()),
		grpc.WithPerRPCCredentials(&bearerAuth{token: "1234"}),
	)
	require.NoError(t, err, "Failed to create gRPC client")
	t.Cleanup(func() {
		_ = conn.Close()
		if t.Failed() {
			report.setResult(t.Name(), "FAIL")
		} else {
			report.setResult(t.Name(), "PASS")
		}
	})
	conn.Connect()
	return pb.NewKesselInventoryServiceClient(conn)
}

func uniqueID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

// --- DB snapshot for report ---

func snapshotDBState(localResourceId, reporterType, resourceType string) string {
	var rr datamodel.ReporterResource
	err := db.Where("local_resource_id = ? AND reporter_type = ? AND resource_type = ?",
		localResourceId, reporterType, resourceType).
		First(&rr).Error
	if err != nil {
		return "not found"
	}
	return fmt.Sprintf("ver=%d gen=%d tombstone=%v", rr.RepresentationVersion, rr.Generation, rr.Tombstone)
}

// --- API action helpers ---

func reportResource(t *testing.T, client pb.KesselInventoryServiceClient,
	resourceType, reporterType, instanceId, localResourceId, workspaceId string,
) {
	t.Helper()
	reporterStruct, err := structpb.NewStruct(map[string]interface{}{
		"ansible_host": "test-host.example.com",
	})
	require.NoError(t, err)

	_, err = client.ReportResource(context.Background(), &pb.ReportResourceRequest{
		WriteVisibility:    pb.WriteVisibility_MINIMIZE_LATENCY,
		Type:               resourceType,
		ReporterType:       reporterType,
		ReporterInstanceId: instanceId,
		Representations: &pb.ResourceRepresentations{
			Metadata: &pb.RepresentationMetadata{
				LocalResourceId: localResourceId,
				ApiHref:         "https://example.com/api",
				ConsoleHref:     proto.String("https://example.com/console"),
				ReporterVersion: proto.String("0.1"),
			},
			Common: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"workspace_id": structpb.NewStringValue(workspaceId),
				},
			},
			Reporter: reporterStruct,
		},
	})
	require.NoError(t, err, "ReportResource failed")

	dbState := snapshotDBState(localResourceId, reporterType, resourceType)
	report.addStep(t.Name(),
		fmt.Sprintf("ReportResource %s/%s (%s, workspace=%s)", resourceType, localResourceId, reporterType, workspaceId),
		dbState, "")
}

func deleteResource(t *testing.T, client pb.KesselInventoryServiceClient,
	resourceType, localResourceId, reporterType, instanceId string,
) {
	t.Helper()
	_, err := client.DeleteResource(context.Background(), &pb.DeleteResourceRequest{
		Reference: &pb.ResourceReference{
			ResourceType: resourceType,
			ResourceId:   localResourceId,
			Reporter: &pb.ReporterReference{
				Type:       reporterType,
				InstanceId: proto.String(instanceId),
			},
		},
	})
	require.NoError(t, err, "DeleteResource failed")

	dbState := snapshotDBState(localResourceId, reporterType, resourceType)
	report.addStep(t.Name(),
		fmt.Sprintf("DeleteResource %s/%s (%s)", resourceType, localResourceId, reporterType),
		dbState, "")
}

func makeCheckReq(resourceType, resourceId, reporterType string, instanceId *string, workspaceId string) *pb.CheckRequest {
	reporter := &pb.ReporterReference{Type: reporterType}
	if instanceId != nil {
		reporter.InstanceId = instanceId
	}
	return &pb.CheckRequest{
		Object: &pb.ResourceReference{
			ResourceType: resourceType,
			ResourceId:   resourceId,
			Reporter:     reporter,
		},
		Relation: "workspace",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspaceId,
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}
}

func makeCheckForUpdateReq(resourceType, resourceId, reporterType string, instanceId *string, workspaceId string) *pb.CheckForUpdateRequest {
	reporter := &pb.ReporterReference{Type: reporterType}
	if instanceId != nil {
		reporter.InstanceId = instanceId
	}
	return &pb.CheckForUpdateRequest{
		Object: &pb.ResourceReference{
			ResourceType: resourceType,
			ResourceId:   resourceId,
			Reporter:     reporter,
		},
		Relation: "workspace",
		Subject: &pb.SubjectReference{
			Resource: &pb.ResourceReference{
				ResourceType: "workspace",
				ResourceId:   workspaceId,
				Reporter:     &pb.ReporterReference{Type: "rbac"},
			},
		},
	}
}

const (
	maxConsecutiveConnErrors = 5
	defaultPollTimeout       = 30
)

func checkActionLabel(req *pb.CheckRequest) string {
	return fmt.Sprintf("Check %s/%s (workspace=%s)",
		req.GetObject().GetResourceType(),
		req.GetObject().GetResourceId(),
		req.GetSubject().GetResource().GetResourceId())
}

func pollCheck(t *testing.T, client pb.KesselInventoryServiceClient,
	req *pb.CheckRequest, expected pb.Allowed, timeoutSec int,
) *pb.ConsistencyToken {
	t.Helper()
	ctx := context.Background()
	connErrors := 0
	label := checkActionLabel(req)

	for i := 0; i < timeoutSec; i++ {
		resp, err := client.Check(ctx, req)
		if err == nil && resp.GetAllowed() == expected {
			t.Logf("Check returned %v on attempt %d", expected, i+1)
			report.addStep(t.Name(), label, "",
				fmt.Sprintf("%v (attempt %d)", expected, i+1))
			return resp.GetConsistencyToken()
		}
		if err != nil {
			t.Logf("Check attempt %d: %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					report.addStep(t.Name(), label, "",
						fmt.Sprintf("UNREACHABLE after %d conn errors", connErrors))
					t.Fatalf("Service unreachable after %d consecutive connection errors", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("Check attempt %d: got %v, want %v", i+1, resp.GetAllowed(), expected)
		}
		time.Sleep(1 * time.Second)
	}
	report.addStep(t.Name(), label, "",
		fmt.Sprintf("TIMEOUT waiting for %v after %ds", expected, timeoutSec))
	t.Fatalf("Check did not return %v within %ds", expected, timeoutSec)
	return nil
}

func checkForUpdateActionLabel(req *pb.CheckForUpdateRequest) string {
	return fmt.Sprintf("CheckForUpdate %s/%s (workspace=%s)",
		req.GetObject().GetResourceType(),
		req.GetObject().GetResourceId(),
		req.GetSubject().GetResource().GetResourceId())
}

func pollCheckForUpdate(t *testing.T, client pb.KesselInventoryServiceClient,
	req *pb.CheckForUpdateRequest, expected pb.Allowed, timeoutSec int,
) *pb.CheckForUpdateResponse {
	t.Helper()
	ctx := context.Background()
	connErrors := 0
	label := checkForUpdateActionLabel(req)

	for i := 0; i < timeoutSec; i++ {
		resp, err := client.CheckForUpdate(ctx, req)
		if err == nil && resp.GetAllowed() == expected {
			t.Logf("CheckForUpdate returned %v on attempt %d", expected, i+1)
			tokenInfo := ""
			if resp.GetConsistencyToken() != nil && resp.GetConsistencyToken().GetToken() != "" {
				tokenInfo = " +token"
			}
			report.addStep(t.Name(), label, "",
				fmt.Sprintf("%v (attempt %d)%s", expected, i+1, tokenInfo))
			return resp
		}
		if err != nil {
			t.Logf("CheckForUpdate attempt %d: %v", i+1, err)
			if isConnectionError(err) {
				connErrors++
				if connErrors >= maxConsecutiveConnErrors {
					report.addStep(t.Name(), label, "",
						fmt.Sprintf("UNREACHABLE after %d conn errors", connErrors))
					t.Fatalf("Service unreachable after %d consecutive connection errors", connErrors)
				}
			}
		} else {
			connErrors = 0
			t.Logf("CheckForUpdate attempt %d: got %v, want %v", i+1, resp.GetAllowed(), expected)
		}
		time.Sleep(1 * time.Second)
	}
	report.addStep(t.Name(), label, "",
		fmt.Sprintf("TIMEOUT waiting for %v after %ds", expected, timeoutSec))
	t.Fatalf("CheckForUpdate did not return %v within %ds", expected, timeoutSec)
	return nil
}

func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "transport is closing")
}

// recordBulkResult records a bulk check result in the report without polling.
func recordBulkResult(t *testing.T, action string, results []string) {
	t.Helper()
	report.addStep(t.Name(), action, "", strings.Join(results, ", "))
}

// recordError records an expected error in the report.
func recordError(t *testing.T, action string, err error) {
	t.Helper()
	report.addStep(t.Name(), action, "", fmt.Sprintf("error: %v", err))
}

// --- DB assertion helpers ---

func findReporterResource(t *testing.T, localResourceId, reporterType, resourceType string) datamodel.ReporterResource {
	t.Helper()
	var rr datamodel.ReporterResource
	err := db.Where("local_resource_id = ? AND reporter_type = ? AND resource_type = ?",
		localResourceId, reporterType, resourceType).
		First(&rr).Error
	require.NoError(t, err, "reporter_resources row not found for %s/%s/%s", reporterType, resourceType, localResourceId)
	return rr
}

func findResourceByReporterKey(t *testing.T, localResourceId, reporterType, resourceType string) datamodel.Resource {
	t.Helper()
	rr := findReporterResource(t, localResourceId, reporterType, resourceType)
	var res datamodel.Resource
	err := db.Where("id = ?", rr.ResourceID).First(&res).Error
	require.NoError(t, err, "resource row not found for id %s", rr.ResourceID)
	return res
}

func assertReporterResource(t *testing.T, localResourceId, reporterType, resourceType string,
	expectedRepVersion, expectedGeneration uint, expectedTombstone bool,
) {
	t.Helper()
	rr := findReporterResource(t, localResourceId, reporterType, resourceType)
	assert.Equal(t, expectedRepVersion, rr.RepresentationVersion, "representation_version mismatch")
	assert.Equal(t, expectedGeneration, rr.Generation, "generation mismatch")
	assert.Equal(t, expectedTombstone, rr.Tombstone, "tombstone mismatch")
}

func assertReporterRepresentation(t *testing.T, reporterResourceID uuid.UUID,
	expectedVersion, expectedGeneration uint, expectedTombstone bool,
) {
	t.Helper()
	var rep datamodel.ReporterRepresentation
	err := db.Where("reporter_resource_id = ? AND version = ? AND generation = ?",
		reporterResourceID, expectedVersion, expectedGeneration).
		First(&rep).Error
	require.NoError(t, err, "reporter_representations row not found for rrID=%s version=%d gen=%d",
		reporterResourceID, expectedVersion, expectedGeneration)
	assert.Equal(t, expectedTombstone, rep.Tombstone, "reporter_representations tombstone mismatch")
}

func assertCommonRepresentation(t *testing.T, resourceID uuid.UUID, expectedVersion uint, expectedWorkspaceId string) {
	t.Helper()
	var cr datamodel.CommonRepresentation
	err := db.Where("resource_id = ? AND version = ?", resourceID, expectedVersion).
		First(&cr).Error
	require.NoError(t, err, "common_representations row not found for resourceID=%s version=%d",
		resourceID, expectedVersion)

	raw, err := json.Marshal(cr.Data)
	require.NoError(t, err)
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, expectedWorkspaceId, data["workspace_id"], "workspace_id mismatch in common_representations")
}
