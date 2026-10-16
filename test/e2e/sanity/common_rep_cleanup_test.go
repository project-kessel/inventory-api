//go:build sanity

package sanity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/gorm"
)

const cleanupFixtureCursor = "ffffff53-3300-4000-8000-000000000000"

// TestSanity_CommonRepCleanup validates the actual Job template under live traffic.
// Opt in with PR1533_NAMESPACE, PR1533_IMAGE, and PR1533_CONFIG_SECRET. The UUID
// suffix is reserved exclusively for this run, verified empty before seeding.
func TestSanity_CommonRepCleanup(t *testing.T) {
	namespace := os.Getenv("PR1533_NAMESPACE")
	if namespace == "" {
		t.Skip("set PR1533_NAMESPACE to run cleanup validation")
	}
	require.Equal(t, "ephemeral-kkotzy", namespace)
	image := os.Getenv("PR1533_IMAGE")
	configSecret := os.Getenv("PR1533_CONFIG_SECRET")
	require.NotEmpty(t, image)
	require.NotEmpty(t, configSecret)
	cluster, err := exec.Command("oc", "whoami", "--show-server").Output()
	require.NoError(t, err)
	require.Contains(t, string(cluster), "crc-eph")
	var existing int64
	require.NoError(t, db.Table("resource").Where("id > ?", cleanupFixtureCursor).Count(&existing).Error)
	require.Zero(t, existing, "refusing to prune a UUID suffix containing non-test resources")

	client := newClient(t)
	relations := cleanupRelations(t)
	instance := uniqueID("pr1533")
	resourceType, err := bizmodel.NewResourceType("host")
	require.NoError(t, err)
	reporterType, err := bizmodel.NewReporterType("hbi")
	require.NoError(t, err)
	apiIDs := make([]string, 4)
	// Clean only the UUID range proven empty above. Reporter deletion cascades
	// its history; no global orphan sweep or unrelated tuple deletion occurs.
	t.Cleanup(func() {
		for _, id := range apiIDs {
			if id != "" {
				var rootID, workspace string
				_ = db.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ? AND reporter_instance_id = ?", id, instance).Scan(&rootID).Error
				_ = db.Raw("SELECT data->>'workspace_id' FROM common_representations WHERE resource_id = ? ORDER BY version DESC LIMIT 1", rootID).Scan(&workspace).Error
				deleteResource(t, client, "host", id, "hbi", instance)
				if workspace != "" {
					pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), workspace), pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
				}
				localID, err := bizmodel.NewLocalResourceId(id)
				require.NoError(t, err)
				filter := bizmodel.NewTupleFilter().WithObjectType(resourceType).WithReporterType(reporterType).WithObjectId(localID)
				_, err = relations.DeleteTuples(context.Background(), filter, nil)
				assert.NoError(t, err)
				_ = db.Exec("DELETE FROM reporter_resources WHERE resource_id = ?", rootID).Error
				_ = db.Exec("DELETE FROM common_representations WHERE resource_id = ?", rootID).Error
				_ = db.Exec("DELETE FROM resource WHERE id = ?", rootID).Error
			}
		}
		_ = db.Exec("DELETE FROM reporter_resources WHERE resource_id > ?", cleanupFixtureCursor).Error
		_ = db.Exec("DELETE FROM common_representations WHERE resource_id > ?", cleanupFixtureCursor).Error
		_ = db.Exec("DELETE FROM resource WHERE id > ?", cleanupFixtureCursor).Error
	})

	// 25k roots x six rows give exactly 100k prunable historical rows.
	require.NoError(t, db.Exec(`INSERT INTO resource (id, type, common_version)
  SELECT ('ffffff53-3300-4000-8000-' || lpad(i::text,12,'0'))::uuid, 'host', CASE WHEN i%2=0 THEN NULL ELSE 5 END
  FROM generate_series(1,25000) i`).Error)
	require.NoError(t, db.Exec(`INSERT INTO common_representations
  (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at)
  SELECT id,v,jsonb_build_object('workspace_id', 'bulk'), 'hbi', ?, id::text || '-' || v, now()
  FROM resource CROSS JOIN generate_series(0,5) v WHERE id > ?`, instance, cleanupFixtureCursor).Error)
	require.NoError(t, db.Exec(`INSERT INTO reporter_resources
  (id,local_resource_id,reporter_type,resource_type,reporter_instance_id,resource_id,api_href,representation_version,generation,tombstone)
  SELECT id,id::text,'hbi','host',?,id,'https://example.test',0,0,false FROM resource WHERE id > ?`, instance, cleanupFixtureCursor).Error)

	// Create genuine API histories, drain their initial events, then move only
	// these test roots into the isolated suffix. Tuple identities use reporter keys.
	for index := range apiIDs {
		id := uniqueID("pr1533-api")
		apiIDs[index] = id
		for version := 0; version < 4; version++ {
			workspace := fmt.Sprintf("%s-ws-%d", id, version)
			reportResource(t, client, "host", "hbi", instance, id, workspace)
			pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), workspace), pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
			if version > 0 {
				pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), fmt.Sprintf("%s-ws-%d", id, version-1)), pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
			}
		}
		newRoot := fmt.Sprintf("ffffff53-3300-4000-8000-%012d", 100001+index)
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			var oldRoot string
			if err := tx.Raw("SELECT resource_id FROM reporter_resources WHERE local_resource_id = ? AND reporter_instance_id = ?", id, instance).Scan(&oldRoot).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE resource SET id = ? WHERE id = ?", newRoot, oldRoot).Error; err != nil {
				return err
			}
			if err := tx.Exec("UPDATE common_representations SET resource_id = ? WHERE resource_id = ?", newRoot, oldRoot).Error; err != nil {
				return err
			}
			return tx.Exec("UPDATE reporter_resources SET resource_id = ? WHERE resource_id = ?", newRoot, oldRoot).Error
		}))
	}
	job := func(t *testing.T, execute bool, batchSize, maxBatches int, cursor string) string {
		t.Helper()
		name := fmt.Sprintf("pr1533-%d", time.Now().UnixNano())
		split := strings.LastIndex(image, ":")
		require.Greater(t, split, 0)
		args := []string{"process", "--local", "-f", "../../../deploy/kessel-inventory-common-representations-cleanup-job.yaml",
			"-p", "JOB_NAME=" + name, "-p", "INVENTORY_IMAGE=" + image[:split], "-p", "IMAGE_TAG=" + image[split+1:],
			"-p", "INVENTORY_API_CONFIG_NAME=" + configSecret, "-p", "EXECUTE=" + strconv.FormatBool(execute),
			"-p", "BATCH_SIZE=" + strconv.Itoa(batchSize), "-p", "MAX_BATCHES=" + strconv.Itoa(maxBatches),
			"-p", "MAX_DURATION=8m", "-p", "ACTIVE_DEADLINE_SECONDS=540", "-p", "BATCH_DELAY=100ms",
			"-p", "RESOURCE_TYPE=host", "-p", "START_AFTER=" + cursor}
		rendered, err := exec.Command("oc", args...).Output()
		require.NoError(t, err)
		path := filepath.Join(t.TempDir(), name+".json")
		require.NoError(t, os.WriteFile(path, rendered, 0600))
		t.Cleanup(func() { _ = exec.Command("oc", "delete", "job", name, "-n", namespace, "--ignore-not-found").Run() })
		output, err := exec.Command("oc", "apply", "-f", path, "-n", namespace).CombinedOutput()
		require.NoError(t, err, "%s", output)
		require.Eventually(t, func() bool {
			data, err := exec.Command("oc", "get", "job", name, "-n", namespace, "-o", "json").Output()
			if err != nil {
				return false
			}
			var obj struct {
				Status struct {
					Succeeded int
					Failed    int
				}
			}
			if json.Unmarshal(data, &obj) != nil {
				return false
			}
			return obj.Status.Succeeded > 0 || obj.Status.Failed > 0
		}, 9*time.Minute, 2*time.Second)
		logs, err := exec.Command("oc", "logs", "job/"+name, "-n", namespace).CombinedOutput()
		require.NoError(t, err)
		state, err := exec.Command("oc", "get", "job", name, "-n", namespace, "-o", "jsonpath={.status.succeeded}").Output()
		require.NoError(t, err)
		require.Equal(t, "1", string(state), "%s", logs)
		t.Logf("job %s completed", name)
		return string(logs)
	}
	recordCleanupPlan(t, cleanupFixtureCursor)
	preview := job(t, false, 1000, 1, cleanupFixtureCursor)
	assert.Equal(t, int64(100008), cleanupLogCount(t, preview, "prunable_common_representations"))
	var before int64
	require.NoError(t, db.Table("common_representations").Where("resource_id > ?", cleanupFixtureCursor).Count(&before).Error)
	assert.Equal(t, int64(150016), before)
	limited := job(t, true, 4, 1, cleanupFixtureCursor)
	assert.Equal(t, int64(4), cleanupLogCount(t, limited, "common_representations_deleted"))
	require.Contains(t, limited, "pass_complete=false")

	// Independent clients keep reads and writes active during the actual prune.
	stop := make(chan struct{})
	var stopOnce sync.Once
	var workers sync.WaitGroup
	stopWorkers := func() { stopOnce.Do(func() { close(stop) }); workers.Wait() }
	defer stopWorkers()
	var mutex sync.Mutex
	var reports, checks, failures int
	var maxRequestDuration time.Duration
	latest := make([]string, len(apiIDs))
	started := time.Now()
	for index, id := range apiIDs {
		workers.Add(1)
		go func(index int, id string) {
			defer workers.Done()
			sequence := 4
			for {
				select {
				case <-stop:
					return
				default:
				}
				workspace := fmt.Sprintf("%s-ws-%d", id, sequence)
				request := &pb.ReportResourceRequest{Type: "host", ReporterType: "hbi", ReporterInstanceId: instance, Representations: &pb.ResourceRepresentations{
					Metadata: &pb.RepresentationMetadata{LocalResourceId: id, ApiHref: "https://example.test"},
					Common:   &structpb.Struct{Fields: map[string]*structpb.Value{"workspace_id": structpb.NewStringValue(workspace)}},
					Reporter: &structpb.Struct{Fields: map[string]*structpb.Value{"ansible_host": structpb.NewStringValue("test.example")}},
				}}
				began := time.Now()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				_, err := client.ReportResource(ctx, request)
				if err == nil {
					_, err = client.Check(ctx, cleanupCheckReq("host", id, "hbi", proto.String(instance), workspace))
				}
				cancel()
				mutex.Lock()
				reports++
				checks++
				if err != nil {
					failures++
				}
				latest[index] = workspace
				mutex.Unlock()
				mutex.Lock()
				maxRequestDuration = max(maxRequestDuration, time.Since(began))
				mutex.Unlock()
				sequence++
				time.Sleep(25 * time.Millisecond)
			}
		}(index, id)
	}
	output := job(t, true, 1000, 1000, cleanupFixtureCursor)
	stopWorkers()
	t.Logf("live cleanup elapsed=%s reports=%d checks=%d failures=%d max_report_check_duration=%s deleted=%d", time.Since(started), reports, checks, failures, maxRequestDuration, cleanupLogCount(t, output, "common_representations_deleted"))
	require.Zero(t, failures)
	require.Greater(t, reports, 20)
	require.Contains(t, output, "pass_complete=true")
	var bulkRows int64
	require.NoError(t, db.Table("common_representations").Where("resource_id > ? AND resource_id < ?", cleanupFixtureCursor, "ffffff53-3300-4000-8000-000000100000").Count(&bulkRows).Error)
	assert.Equal(t, int64(50000), bulkRows)
	for index, id := range apiIDs {
		pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), latest[index]), pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
		// Explicitly check a formerly revoked relationship; successful requests alone
		// are not treated as proof that every historical authorization was removed.
		response, err := client.Check(context.Background(), cleanupCheckReq("host", id, "hbi", proto.String(instance), id+"-ws-0"))
		require.NoError(t, err)
		assert.Equal(t, pb.Allowed_ALLOWED_FALSE, response.GetAllowed())
	}
	_ = job(t, true, 1000, 1000, cleanupFixtureCursor)
	id := apiIDs[0]
	deleteResource(t, client, "host", id, "hbi", instance)
	pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), latest[0]), pb.Allowed_ALLOWED_FALSE, defaultPollTimeout)
	reportResource(t, client, "host", "hbi", instance, id, id+"-revived")
	pollCheck(t, client, cleanupCheckReq("host", id, "hbi", proto.String(instance), id+"-revived"), pb.Allowed_ALLOWED_TRUE, defaultPollTimeout)
	_ = job(t, true, 1000, 1000, cleanupFixtureCursor)
	final := job(t, true, 1000, 1000, cleanupFixtureCursor)
	assert.Zero(t, cleanupLogCount(t, final, "common_representations_deleted"))
}

func cleanupLogCount(t *testing.T, output, field string) int64 {
	t.Helper()
	matches := regexp.MustCompile(field+`=(\d+)`).FindAllStringSubmatch(output, -1)
	require.NotEmpty(t, matches, "missing %s in %s", field, output)
	value, err := strconv.ParseInt(matches[len(matches)-1][1], 10, 64)
	require.NoError(t, err)
	return value
}

func cleanupCheckReq(resourceType, resourceID, reporterType string, instanceID *string, workspaceID string) *pb.CheckRequest {
	request := makeCheckReq(resourceType, resourceID, reporterType, instanceID, workspaceID)
	request.Consistency = &pb.Consistency{Requirement: &pb.Consistency_AtLeastAsAcknowledged{AtLeastAsAcknowledged: true}}
	return request
}
