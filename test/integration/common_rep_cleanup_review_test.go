//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/cleanup"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/consumer/retry"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

type pr1533ReviewSQLCapture struct {
	gormlogger.Interface
	classificationSQL string
}

func TestCommonRepCleanupPendingHistoryLimitation(t *testing.T) {
	for _, prune := range []bool{false, true} {
		t.Run(fmt.Sprintf("prune-%t", prune), func(t *testing.T) {
			pr1533ReviewConsumerWorkspaceResidue(t, prune)
		})
	}
}

func pr1533ReviewConsumerWorkspaceResidue(t *testing.T, prune bool) {
	db, _ := setupCommonRepCleanupPostgres(t)
	ctx := context.Background()
	rootID := uuid.New()
	seedCommonRepRoot(t, db, rootID, "host", uintPointer(0), 0)
	require.NoError(t, insertCommonRepReporter(t, db, rootID, "review-consumer", "host", "hbi", false).Error)
	require.NoError(t, db.Exec("UPDATE common_representations SET data = '{\"workspace_id\":\"old-workspace\"}'::jsonb WHERE resource_id = ?", rootID).Error)
	metrics := metricscollector.NewFakeMetricsCollector()
	resourceRepository := data.NewResourceRepository(db, data.NewGormTransactionManager(metrics, 5), data.SetOutboxPublisher(storage.OutboxModeNone))
	relations := data.NewSimpleRelationsRepository()
	logger := log.NewHelper(log.DefaultLogger)
	worker := &consumer.InventoryConsumer{
		ResourceRepository: resourceRepository,
		Relations:          relations,
		MetricsCollector:   metrics,
		Logger:             logger,
		RetryOptions:       retry.NewOptions(),
		SchemaService:      bizmodel.NewSchemaService(data.NewInMemorySchemaRepository(), logger),
	}
	process := func(version int, operation string) {
		message := &kafka.Message{Value: []byte(fmt.Sprintf(`{"schema":{"type":"string"},"payload":{"reporter_resource_key":{"local_resource_id":"review-consumer","resource_type":"host","reporter":{"reporter_type":"hbi","reporter_instance_id":"cleanup-instance"}},"common_version":%d}}`, version))}
		_, err := worker.ProcessMessage(map[string]string{"operation": operation, "txid": fmt.Sprintf("review-event-%d", version)}, true, message)
		require.NoError(t, err)
	}
	key := commonRepReporterKey(t, "hbi", "review-consumer")
	checkWorkspace := func(workspace string) bool {
		tuple := bizmodel.NewWorkspaceRelationsTuple(workspace, key)
		result, err := relations.Check(ctx, bizmodel.NewRelationship(tuple.Object(), tuple.Relation(), tuple.Subject()), bizmodel.NewConsistencyMinimizeLatency())
		require.NoError(t, err)
		return result.Allowed()
	}
	process(0, "created")
	require.True(t, checkWorkspace("old-workspace"))
	for version := uint(1); version <= 2; version++ {
		require.NoError(t, insertCommonRepRow(t, db, rootID, version, fmt.Sprintf("review-write-%d", version)).Error)
	}
	require.NoError(t, db.Exec("UPDATE common_representations SET data = '{\"workspace_id\":\"new-workspace\"}'::jsonb WHERE resource_id = ? AND version > 0", rootID).Error)
	require.NoError(t, db.Exec("UPDATE resource SET common_version = 2 WHERE id = ?", rootID).Error)
	policy, err := bizmodel.NewCommonRepCleanupPolicy(2, "")
	require.NoError(t, err)
	options, err := cleanup.NewOptions(20000, 1000, 0, 10, time.Minute)
	require.NoError(t, err)
	if prune {
		result, err := newTestCommonRepCleanupUsecase(t, db).Execute(ctx, policy, options, uuid.Nil, nil, nil)
		require.NoError(t, err)
		require.Equal(t, int64(1), result.DeletedRows)
	}
	process(1, "updated")
	process(2, "updated")
	require.True(t, checkWorkspace("new-workspace"))
	// Accepted limitation: pruning a required predecessor loses the removal
	// delta even with only two pending events. This must not be described as
	// authorization-safe merely because ProcessMessage succeeds.
	require.Equal(t, prune, checkWorkspace("old-workspace"), "only the unpruned control revokes the old workspace")
}

func (logger *pr1533ReviewSQLCapture) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.Contains(sql, "WITH chunk AS") {
		logger.classificationSQL = sql
	}
}

func TestCommonRepCleanupChunkPlans(t *testing.T) {
	db, _ := setupCommonRepCleanupPostgres(t)
	for _, scenario := range []struct {
		name    string
		pointer string
		roots   int
		mixed   bool
	}{
		{"explicit-20000", "0", 20000, false},
		{"null-1000", "NULL", 1000, false},
		{"null-20000", "NULL", 20000, false},
		{"mixed-deep-20000", "NULL", 20000, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			isolateCommonRepCleanupPostgresSubtest(t, db)
			require.NoError(t, db.Exec(fmt.Sprintf(`INSERT INTO resource (id, type, common_version)
				SELECT ('10000000-0000-0000-0000-' || lpad(i::text, 12, '0'))::uuid, 'host', %s
				FROM generate_series(1, %d) i`, scenario.pointer, scenario.roots)).Error)
			require.NoError(t, db.Exec(`INSERT INTO common_representations
				(resource_id, version, data, reported_by_reporter_type, reported_by_reporter_instance, created_at)
				SELECT id, 0, '{"workspace_id":"review"}'::jsonb, 'hbi', 'review', now() FROM resource`).Error)
			require.NoError(t, db.Exec(`INSERT INTO reporter_resources
				(id, local_resource_id, reporter_type, resource_type, reporter_instance_id, resource_id, api_href, representation_version, generation, tombstone)
				SELECT id, id::text, 'hbi', 'host', 'review', id, 'https://example.test/review', 0, 0, false FROM resource`).Error)
			chunkRows := 20000
			if scenario.mixed {
				require.NoError(t, db.Exec(`INSERT INTO common_representations
                    (resource_id,version,data,reported_by_reporter_type,reported_by_reporter_instance,created_at)
                    SELECT r.id,v,'{"workspace_id":"review"}'::jsonb,'hbi','review',now()
                    FROM (SELECT id FROM resource ORDER BY id LIMIT 100) r
                    CROSS JOIN generate_series(1,1000) v`).Error)
				chunkRows = 200000
			}
			for _, table := range []string{"common_representations", "resource", "reporter_resources"} {
				require.NoError(t, db.Exec("ANALYZE "+table).Error)
			}
			capture := &pr1533ReviewSQLCapture{Interface: gormlogger.Discard}
			transactions := data.NewCommonRepCleanupTransaction(db.Session(&gorm.Session{Logger: capture}), data.NewGormTransactionManager(metricscollector.NewFakeMetricsCollector(), 5))
			policy, err := bizmodel.NewCommonRepCleanupPolicy(2, "")
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Second)
			defer cancel()
			started := time.Now()
			var roots []bizmodel.CommonRepRootClassification
			err = transactions.WithinSerializableCommonRepCleanup(ctx, "cleanup_plan_test", true, 30*time.Second, 5*time.Second, func(repository bizmodel.CommonRepCleanupRepository) error {
				var readErr error
				roots, _, _, readErr = repository.ReadChunk(ctx, policy, uuid.Nil, chunkRows)
				return readErr
			})
			require.NoError(t, err)
			t.Logf("ReadChunk roots=%d elapsed=%s", len(roots), time.Since(started))
			var planJSON string
			require.NoError(t, db.WithContext(ctx).Raw("EXPLAIN (ANALYZE, BUFFERS, TIMING OFF, FORMAT JSON) "+capture.classificationSQL).Scan(&planJSON).Error)
			var plans []map[string]any
			require.NoError(t, json.Unmarshal([]byte(planJSON), &plans))
			var checkPlan func(map[string]any)
			checkPlan = func(node map[string]any) {
				if node["Node Type"] == "CTE Scan" && node["CTE Name"] == "chunk" {
					require.LessOrEqual(t, node["Actual Loops"].(float64), float64(1), "whole-chunk scans must not execute once per root")
				}
				if children, ok := node["Plans"].([]any); ok {
					for _, child := range children {
						checkPlan(child.(map[string]any))
					}
				}
			}
			checkPlan(plans[0]["Plan"].(map[string]any))
			t.Logf("EXPLAIN execution time: %v ms", plans[0]["Execution Time"])
			require.Len(t, roots, scenario.roots)
			var prunableRows int64
			for _, root := range roots {
				prunableRows += root.PrunableRows()
			}
			expected := int64(0)
			if scenario.mixed {
				expected = 99900
			}
			require.Equal(t, expected, prunableRows)

		})
	}
}
