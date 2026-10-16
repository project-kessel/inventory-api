//go:build sanity

package sanity

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func cleanupRelations(t *testing.T) bizmodel.RelationsRepository {
	t.Helper()
	endpoint := os.Getenv("PR1533_RELATIONS_URL")
	require.NotEmpty(t, endpoint)
	config, errs := kessel.NewConfig(&kessel.Options{URL: endpoint, Insecure: true}).Complete(context.Background())
	require.Empty(t, errs)
	t.Cleanup(func() { _ = config.GetGRPCConn().Close() })
	relations, err := data.NewGRPCRelationsRepository(context.Background(), config, log.NewHelper(log.DefaultLogger))
	require.NoError(t, err)
	return relations
}

// TestSanity_CommonRepCleanupPendingHistoryLimitation uses real ephemeral
// PostgreSQL and relations storage without emitting fixture events to the shared
// topic. Only the fixture's events are driven through the real consumer method;
// the running API, Kafka pipeline, and normal consumer remain available.
func TestSanity_CommonRepCleanupPendingHistoryLimitation(t *testing.T) {
	if os.Getenv("PR1533_NAMESPACE") == "" {
		t.Skip("set PR1533_NAMESPACE to run cleanup validation")
	}
	require.Equal(t, "ephemeral-kkotzy", os.Getenv("PR1533_NAMESPACE"))
	relations := cleanupRelations(t)
	for _, prune := range []bool{false, true} {
		t.Run(fmt.Sprintf("prune-%t", prune), func(t *testing.T) {
			ctx := context.Background()
			rootID := uuid.New()
			localID := uniqueID("pr1533-limitation")
			reporterID := uuid.New()
			resourceType, err := bizmodel.NewResourceType("host")
			require.NoError(t, err)
			reporterType, err := bizmodel.NewReporterType("hbi")
			require.NoError(t, err)
			local, err := bizmodel.NewLocalResourceId(localID)
			require.NoError(t, err)
			instance, err := bizmodel.NewReporterInstanceId("pr1533-limitation")
			require.NoError(t, err)
			key, err := bizmodel.NewReporterResourceKey(local, resourceType, reporterType, instance)
			require.NoError(t, err)
			t.Cleanup(func() {
				_, _ = relations.DeleteTuples(ctx, bizmodel.NewTupleFilter().WithObjectType(resourceType).WithReporterType(reporterType).WithObjectId(local), nil)
				lockType, err := bizmodel.NewResourceType("lock")
				require.NoError(t, err)
				lockReporter, err := bizmodel.NewReporterType("kessel")
				require.NoError(t, err)
				lockID, err := bizmodel.NewLocalResourceId(localID + "/0")
				require.NoError(t, err)
				_, err = relations.DeleteTuples(ctx, bizmodel.NewTupleFilter().WithObjectType(lockType).WithReporterType(lockReporter).WithObjectId(lockID), nil)
				require.NoError(t, err)
				_ = db.Exec("DELETE FROM reporter_resources WHERE id = ?", reporterID).Error
				_ = db.Exec("DELETE FROM common_representations WHERE resource_id = ?", rootID).Error
				_ = db.Exec("DELETE FROM resource WHERE id = ?", rootID).Error
			})
			require.NoError(t, db.Exec("INSERT INTO resource (id,type,common_version) VALUES (?,'host',0)", rootID).Error)
			require.NoError(t, db.Exec(`INSERT INTO reporter_resources (id,local_resource_id,reporter_type,resource_type,reporter_instance_id,resource_id,api_href,representation_version,generation,tombstone)
    VALUES (?,?,'hbi','host','pr1533-limitation',?,'https://example.test',0,0,false)`, reporterID, localID, rootID).Error)
			insert := func(version int, workspace string) {
				require.NoError(t, db.Exec(`INSERT INTO common_representations (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,transaction_id,created_at)
     VALUES (?,?,jsonb_build_object('workspace_id',?::text),'hbi','pr1533-limitation',?,now())`, rootID, version, workspace, localID+fmt.Sprint(version)).Error)
			}
			insert(0, localID+"-A")
			metrics := metricscollector.NewFakeMetricsCollector()
			logger := log.NewHelper(log.NewStdLogger(io.Discard))
			worker := &consumer.InventoryConsumer{ResourceRepository: data.NewResourceRepository(db, data.NewGormTransactionManager(metrics, 5), data.SetOutboxPublisher(storage.OutboxModeNone)), Relations: relations, MetricsCollector: metrics, Logger: logger, RetryOptions: retry.NewOptions(), SchemaService: bizmodel.NewSchemaService(data.NewInMemorySchemaRepository(), logger)}
			options := consumer.NewOptions()
			options.ConsumerGroupID = localID
			config, errors := consumer.NewConfig(options).Complete()
			require.Empty(t, errors)
			worker.Config = config
			require.NoError(t, worker.RebalanceCallback(nil, kafka.AssignedPartitions{Partitions: []kafka.TopicPartition{{Partition: 0}}}))
			process := func(version int, operation string) {
				message := &kafka.Message{Value: []byte(fmt.Sprintf(`{"schema":{"type":"string"},"payload":{"reporter_resource_key":{"local_resource_id":%q,"resource_type":"host","reporter":{"reporter_type":"hbi","reporter_instance_id":"pr1533-limitation"}},"common_version":%d}}`, localID, version))}
				_, err := worker.ProcessMessage(map[string]string{"operation": operation, "txid": localID + fmt.Sprint(version)}, true, message)
				require.NoError(t, err)
			}
			check := func(workspace string) bool {
				tuple := bizmodel.NewWorkspaceRelationsTuple(workspace, key)
				result, err := relations.CheckForUpdate(ctx, bizmodel.NewRelationship(tuple.Object(), tuple.Relation(), tuple.Subject()))
				require.NoError(t, err)
				return result.Allowed()
			}
			process(0, "created")
			require.True(t, check(localID+"-A"))
			insert(1, localID+"-B")
			insert(2, localID+"-B")
			require.NoError(t, db.Exec("UPDATE resource SET common_version=2 WHERE id = ?", rootID).Error)
			if prune {
				policy, err := bizmodel.NewCommonRepCleanupPolicy(2, "")
				require.NoError(t, err)
				transaction := data.NewCommonRepCleanupTransaction(db, data.NewGormTransactionManager(metrics, 5))
				err = transaction.WithinSerializableCommonRepCleanup(ctx, "ephemeral_history_limitation", false, 30*time.Second, 5*time.Second, func(repository bizmodel.CommonRepCleanupRepository) error {
					impact, err := repository.DeletePrunableRoots(ctx, policy, []bizmodel.ResourceId{bizmodel.DeserializeResourceId(rootID)}, 1000)
					if err == nil {
						require.Equal(t, int64(1), impact.Representations())
					}
					return err
				})
				require.NoError(t, err)
			}
			process(1, "updated")
			process(2, "updated")
			require.True(t, check(localID+"-B"))
			require.Equal(t, prune, check(localID+"-A"))
			t.Logf("prune=%t final_A_authorized=%t final_B_authorized=%t", prune, check(localID+"-A"), check(localID+"-B"))
		})
	}
}

// cleanupPlanCapture observes the actual adapter query rather than duplicating it.
type cleanupPlanCapture struct {
	gormlogger.Interface
	query string
}

func (capture *cleanupPlanCapture) Trace(_ context.Context, _ time.Time, query func() (string, int64), _ error) {
	sql, _ := query()
	if strings.Contains(sql, "WITH chunk AS") {
		capture.query = sql
	}
}
func recordCleanupPlan(t *testing.T, cursor string) {
	t.Helper()
	capture := &cleanupPlanCapture{Interface: gormlogger.Discard}
	scoped := db.Session(&gorm.Session{Logger: capture})
	policy, err := bizmodel.NewCommonRepCleanupPolicy(2, "host")
	require.NoError(t, err)
	transaction := data.NewCommonRepCleanupTransaction(scoped, data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = transaction.WithinSerializableCommonRepCleanup(ctx, "ephemeral_cleanup_plan", true, 30*time.Second, 5*time.Second, func(repository bizmodel.CommonRepCleanupRepository) error {
		_, _, _, err := repository.ReadChunk(ctx, policy, uuid.MustParse(cursor), 20000)
		return err
	})
	require.NoError(t, err)
	var result string
	require.NoError(t, scoped.WithContext(ctx).Raw("EXPLAIN (ANALYZE,BUFFERS,TIMING OFF,FORMAT JSON) "+capture.query).Scan(&result).Error)
	var plans []map[string]any
	require.NoError(t, json.Unmarshal([]byte(result), &plans))
	t.Logf("ephemeral scan EXPLAIN execution=%v ms", plans[0]["Execution Time"])
}
