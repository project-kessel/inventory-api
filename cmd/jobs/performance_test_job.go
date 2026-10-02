package jobs

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
	"gorm.io/gorm"
)

type PerformanceMetrics struct {
	TotalQueries     int64
	SuccessfulQueries int64
	FailedQueries    int64
	Latencies        []time.Duration
	StartTime        time.Time
	EndTime          time.Time
}

func NewPerformanceTestJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	var queryCount int
	var concurrency int
	var testType string

	cmd := &cobra.Command{
		Use:   "performance-test-job",
		Short: "Performance test consumer queries at scale",
		Long: `Stress test the actual consumer read queries from resource_repository.go.

Tests the two main consumer query patterns:
1. FindLatestRepresentations - Gets latest version for a resource
2. FindCurrentAndPreviousVersionedRepresentations - Gets current + previous versions

Measures p50, p95, p99 latencies under load and generates detailed metrics.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPerformanceTest(storageOptions, loggerOptions, queryCount, concurrency, testType)
		},
	}

	cmd.Flags().IntVar(&queryCount, "query-count", 100000, "Total number of queries to execute")
	cmd.Flags().IntVar(&concurrency, "concurrency", 50, "Number of concurrent query workers")
	cmd.Flags().StringVar(&testType, "test-type", "both", "Query type to test: latest, versioned, or both")

	return cmd
}

func runPerformanceTest(storageOptions *storage.Options, loggerOptions common.LoggerOptions, queryCount int, concurrency int, testType string) error {
	_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
	logHelper := log.NewHelper(log.With(logger, "job", "performance_test"))

	storageConfig := storage.NewConfig(storageOptions).Complete()
	db, err := storage.New(storageConfig, logHelper)
	if err != nil {
		return err
	}

	logHelper.Infof("Starting performance test")
	logHelper.Infof("Query count: %d", queryCount)
	logHelper.Infof("Concurrency: %d workers", concurrency)
	logHelper.Infof("Test type: %s", testType)

	// Get sample resource keys for testing
	resourceKeys, err := getSampleResourceKeys(db, 10000)
	if err != nil {
		return fmt.Errorf("failed to get sample resource keys: %w", err)
	}
	logHelper.Infof("Loaded %d sample resource keys for testing", len(resourceKeys))

	// Run performance tests
	var latestMetrics, versionedMetrics *PerformanceMetrics

	if testType == "latest" || testType == "both" {
		logHelper.Info("=== Testing FindLatestRepresentations ===")
		latestMetrics, err = testFindLatestRepresentations(db, logHelper, resourceKeys, queryCount, concurrency)
		if err != nil {
			return fmt.Errorf("latest representations test failed: %w", err)
		}
		printMetrics(logHelper, "FindLatestRepresentations", latestMetrics)
	}

	if testType == "versioned" || testType == "both" {
		logHelper.Info("=== Testing FindCurrentAndPreviousVersionedRepresentations ===")
		versionedMetrics, err = testFindVersionedRepresentations(db, logHelper, resourceKeys, queryCount, concurrency)
		if err != nil {
			return fmt.Errorf("versioned representations test failed: %w", err)
		}
		printMetrics(logHelper, "FindCurrentAndPreviousVersionedRepresentations", versionedMetrics)
	}

	// Print summary
	logHelper.Info("=== Performance Test Summary ===")
	if latestMetrics != nil {
		logHelper.Infof("FindLatestRepresentations - p99: %v, QPS: %.0f",
			calculatePercentile(latestMetrics.Latencies, 99),
			float64(latestMetrics.SuccessfulQueries)/latestMetrics.EndTime.Sub(latestMetrics.StartTime).Seconds())
	}
	if versionedMetrics != nil {
		logHelper.Infof("FindCurrentAndPreviousVersionedRepresentations - p99: %v, QPS: %.0f",
			calculatePercentile(versionedMetrics.Latencies, 99),
			float64(versionedMetrics.SuccessfulQueries)/versionedMetrics.EndTime.Sub(versionedMetrics.StartTime).Seconds())
	}

	return nil
}

type ResourceKey struct {
	LocalResourceID    string
	ReporterType       string
	ResourceType       string
	ReporterInstanceID string
	LatestVersion      uint
}

func getSampleResourceKeys(db *gorm.DB, sampleSize int) ([]ResourceKey, error) {
	var keys []ResourceKey

	// Get random sample of resources with their latest versions
	err := db.Raw(`
		SELECT
			rr.local_resource_id,
			rr.reporter_type,
			rr.resource_type,
			rr.reporter_instance_id,
			rr.representation_version as latest_version
		FROM reporter_resources rr
		WHERE rr.tombstone = false
		ORDER BY RANDOM()
		LIMIT ?
	`, sampleSize).Scan(&keys).Error

	if err != nil {
		return nil, err
	}

	return keys, nil
}

func testFindLatestRepresentations(db *gorm.DB, logHelper *log.Helper, resourceKeys []ResourceKey, queryCount int, concurrency int) (*PerformanceMetrics, error) {
	metrics := &PerformanceMetrics{
		Latencies: make([]time.Duration, 0, queryCount),
		StartTime: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	queriesPerWorker := queryCount / concurrency

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < queriesPerWorker; j++ {
				// Pick random resource key
				key := resourceKeys[rand.Intn(len(resourceKeys))]

				start := time.Now()

				// Execute the actual consumer query
				var result struct {
					Data    internal.JsonObject
					Version uint
				}

				err := db.Table("reporter_resources rr").
					Select("cr.data, cr.version").
					Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id").
					Where("rr.local_resource_id = ? AND rr.reporter_type = ? AND rr.resource_type = ? AND rr.reporter_instance_id = ?",
						key.LocalResourceID, key.ReporterType, key.ResourceType, key.ReporterInstanceID).
					Order("cr.version DESC").
					Limit(1).
					Scan(&result).Error

				latency := time.Since(start)

				mu.Lock()
				atomic.AddInt64(&metrics.TotalQueries, 1)
				if err != nil {
					atomic.AddInt64(&metrics.FailedQueries, 1)
				} else {
					atomic.AddInt64(&metrics.SuccessfulQueries, 1)
					metrics.Latencies = append(metrics.Latencies, latency)
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	metrics.EndTime = time.Now()

	return metrics, nil
}

func testFindVersionedRepresentations(db *gorm.DB, logHelper *log.Helper, resourceKeys []ResourceKey, queryCount int, concurrency int) (*PerformanceMetrics, error) {
	metrics := &PerformanceMetrics{
		Latencies: make([]time.Duration, 0, queryCount),
		StartTime: time.Now(),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	queriesPerWorker := queryCount / concurrency

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < queriesPerWorker; j++ {
				// Pick random resource key
				key := resourceKeys[rand.Intn(len(resourceKeys))]

				start := time.Now()

				// Execute the actual consumer query for current + previous versions
				type commonRepresentationRow struct {
					Data                       internal.JsonObject
					Version                    uint
					ResourceID                 uuid.UUID
					ReportedByReporterType     string
					ReportedByReporterInstance string
					TransactionID              string
				}

				var results []commonRepresentationRow
				cv := key.LatestVersion

				err := db.Table("reporter_resources rr").
					Select("cr.data, cr.version, cr.resource_id, cr.reported_by_reporter_type, cr.reported_by_reporter_instance, cr.transaction_id").
					Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id").
					Where("rr.local_resource_id = ? AND rr.reporter_type = ? AND rr.resource_type = ? AND rr.reporter_instance_id = ?",
						key.LocalResourceID, key.ReporterType, key.ResourceType, key.ReporterInstanceID).
					Where("(cr.version = ? OR cr.version = ?)", cv, cv-1).
					Find(&results).Error

				latency := time.Since(start)

				mu.Lock()
				atomic.AddInt64(&metrics.TotalQueries, 1)
				if err != nil {
					atomic.AddInt64(&metrics.FailedQueries, 1)
				} else {
					atomic.AddInt64(&metrics.SuccessfulQueries, 1)
					metrics.Latencies = append(metrics.Latencies, latency)
				}
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()
	metrics.EndTime = time.Now()

	return metrics, nil
}

func printMetrics(logHelper *log.Helper, testName string, metrics *PerformanceMetrics) {
	duration := metrics.EndTime.Sub(metrics.StartTime)
	qps := float64(metrics.SuccessfulQueries) / duration.Seconds()

	logHelper.Infof("=== %s Results ===", testName)
	logHelper.Infof("Total queries: %d", metrics.TotalQueries)
	logHelper.Infof("Successful: %d", metrics.SuccessfulQueries)
	logHelper.Infof("Failed: %d", metrics.FailedQueries)
	logHelper.Infof("Duration: %v", duration)
	logHelper.Infof("QPS: %.0f queries/sec", qps)
	logHelper.Infof("p50 latency: %v", calculatePercentile(metrics.Latencies, 50))
	logHelper.Infof("p95 latency: %v", calculatePercentile(metrics.Latencies, 95))
	logHelper.Infof("p99 latency: %v", calculatePercentile(metrics.Latencies, 99))
	logHelper.Infof("Min latency: %v", metrics.Latencies[0])
	logHelper.Infof("Max latency: %v", metrics.Latencies[len(metrics.Latencies)-1])
}

func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := int(float64(len(sorted)) * float64(percentile) / 100.0)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}
