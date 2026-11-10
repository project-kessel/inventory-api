package data

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/testutil"
)

// Only execution/access-path fields matter: costs, timings, index names and join
// choices vary across PostgreSQL versions and are not the regression contract.
type representationPlan struct {
	NodeType          string               `json:"Node Type"`
	RelationName      string               `json:"Relation Name"`
	IndexCond         string               `json:"Index Cond"`
	Filter            string               `json:"Filter"`
	ActualRows        float64              `json:"Actual Rows"`
	ActualLoops       float64              `json:"Actual Loops"`
	RowsRemovedFilter float64              `json:"Rows Removed by Filter"`
	Plans             []representationPlan `json:"Plans"`
}

type representationPlanQuery struct {
	sql  string
	vars []interface{}
}

// TestResourceRepositoryRepresentationPlans guards history lookup work using real
// repository SQL. RHCLOUD-52565 / PR #1538 replaced a combined-version predicate
// that filtered common history instead of bounding the composite index lookup.
// Reporter history must likewise be limited before a history-wide join/sort.
// Check access paths and rows visited, not estimated costs or elapsed time.
func TestResourceRepositoryRepresentationPlans(t *testing.T) {
	// SQLite cannot exercise PostgreSQL's parameterized lateral/index plans.
	if !testutil.UsingPostgres() {
		t.Skip("PostgreSQL execution plans require TEST_DB=postgres")
	}
	fixtureStarted := time.Now()
	repository, db := setupResourceRepository(t)
	seedStarted := time.Now()
	resource := createTestResourceWithLocalId(t, "plan-history")
	require.NoError(t, repository.Save(db, resource, bizmodel.OperationTypeCreated, bizmodel.NewTransactionId("plan-history")))
	key := createContractReporterResourceKey(t, "plan-history", "k8s_cluster", "ocm", "ocm-instance-1")

	// Bulk history is large enough to expose unbounded work, without thousands
	// of repository writes or migrations per scenario. Two generations, a later
	// revival and adjacent tombstones also exercise the history bounds.
	resourceSnapshot, reporterResourceSnapshot, _, _, err := resource.Serialize()
	require.NoError(t, err)
	commonHistory := make([]datamodel.CommonRepresentation, 0, 2000)
	for version := uint(1); version <= 2000; version++ {
		representation, err := datamodel.NewCommonRepresentation(resourceSnapshot.ID,
			internal.JsonObject{"workspace_id": fmt.Sprintf("workspace-v%d", version)},
			version, "ocm", "ocm-instance-1", "")
		require.NoError(t, err)
		commonHistory = append(commonHistory, *representation)
	}
	require.NoError(t, db.CreateInBatches(&commonHistory, 500).Error)
	require.NoError(t, db.Where("reporter_resource_id = ?", reporterResourceSnapshot.ID).
		Delete(&datamodel.ReporterRepresentation{}).Error)
	reporterHistory := make([]datamodel.ReporterRepresentation, 0, 2001)
	for history := uint(0); history <= 2000; history++ {
		tombstone := history == 1998 || history == 1999
		data := internal.JsonObject{}
		if !tombstone {
			data["payload"] = fmt.Sprintf("reporter-%d", history)
		}
		representation, err := datamodel.NewReporterRepresentation(data, reporterResourceSnapshot.ID,
			history%1000, history/1000, nil, "", tombstone, nil)
		require.NoError(t, err)
		reporterHistory = append(reporterHistory, *representation)
	}
	require.NoError(t, db.CreateInBatches(&reporterHistory, 500).Error)
	for _, table := range []string{"reporter_resources", "common_representations", "reporter_representations"} {
		require.NoError(t, db.Exec("ANALYZE "+table).Error)
	}
	t.Logf("fixture (private migrated DB + 4001 bulk history rows): %s; seed + ANALYZE: %s", time.Since(fixtureStarted), time.Since(seedStarted))

	// Scan executes the Row callback, not Query. Copy both SQL and parameters
	// before GORM resets the statement; EXPLAIN must replay the real query.
	var captured []representationPlanQuery
	require.NoError(t, db.Callback().Row().After("gorm:row").Register("test:capture_representation_plan", func(tx *gorm.DB) {
		captured = append(captured, representationPlanQuery{
			sql: tx.Statement.SQL.String(), vars: append([]interface{}(nil), tx.Statement.Vars...),
		})
	}))
	t.Cleanup(func() {
		require.NoError(t, db.Callback().Row().Remove("test:capture_representation_plan"))
	})

	planStarted := time.Now()
	var commonQueries, historyQueries []representationPlanQuery
	for _, scenario := range []struct {
		name      string
		versions  bizmodel.RepresentationVersions
		operation bizmodel.EventOperationType
	}{
		{"common update", bizmodel.NewRepresentationVersions(ptrVersion(1999), nil, nil), bizmodel.OperationTypeUpdated},
		{"reporter update", bizmodel.NewRepresentationVersions(nil, ptrVersion(997), ptrGeneration(1)), bizmodel.OperationTypeUpdated},
		{"delete", bizmodel.NewRepresentationVersions(ptrVersion(1999), ptrVersion(999), ptrGeneration(1)), bizmodel.OperationTypeDeleted},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			captured = nil
			current, previous, err := repository.FindCurrentAndPreviousVersionedRepresentations(db, key, scenario.versions, scenario.operation)
			require.NoError(t, err)
			require.NotNil(t, current)
			switch scenario.name {
			case "common update":
				require.NotNil(t, previous)
				require.Equal(t, uint(1999), current.CommonVersion().Uint())
				require.Equal(t, "workspace-v1999", current.WorkspaceID())
				require.Equal(t, uint(1998), previous.CommonVersion().Uint())
				require.Equal(t, "workspace-v1998", previous.WorkspaceID())
				require.Nil(t, current.ReporterData())
				require.Nil(t, previous.ReporterVersion())
			case "reporter update":
				require.NotNil(t, previous)
				require.Equal(t, uint(997), current.ReporterVersion().Uint())
				require.Equal(t, "reporter-1997", current.ReporterData()["payload"])
				require.Equal(t, uint(996), previous.ReporterVersion().Uint())
				require.Equal(t, "reporter-1996", previous.ReporterData()["payload"])
				require.Nil(t, current.CommonVersion())
				require.Nil(t, previous.CommonData())
			case "delete":
				require.Nil(t, previous)
				require.Equal(t, uint(1999), current.CommonVersion().Uint())
				require.Equal(t, "workspace-v1999", current.WorkspaceID())
				require.Equal(t, uint(997), current.ReporterVersion().Uint())
				require.Equal(t, "reporter-1997", current.ReporterData()["payload"])
			}
			queries := append([]representationPlanQuery(nil), captured...)
			require.Len(t, queries, 2, "each scenario must capture both repository lookups")
			for _, query := range queries {
				plan := explainRepresentationQuery(t, db, query)
				if strings.Contains(query.sql, "common_representations") {
					require.NoError(t, validateCommonRepresentationPlan(plan))
					commonQueries = append(commonQueries, query)
				} else if strings.Contains(query.sql, "JOIN LATERAL") {
					require.NoError(t, validateReporterHistoryPlan(plan))
					historyQueries = append(historyQueries, query)
				}
			}
		})
	}
	require.Len(t, commonQueries, 3)
	require.Len(t, historyQueries, 2)

	t.Run("negative controls", func(t *testing.T) {
		// The old combined-version OR can turn version into a filter rather
		// than an index bound. Mutate captured SQL, never maintain a second
		// hand-written copy of a production query.
		query := commonQueries[0]
		predicate := regexp.MustCompile(`cr\.version = (\$[0-9]+)`)
		require.True(t, predicate.MatchString(query.sql))
		query.sql = predicate.ReplaceAllString(query.sql, `(cr.version = ${1} OR cr.version = ${1} - 1)`)
		plan := explainRepresentationQuery(t, db, query)
		require.Error(t, validateCommonRepresentationPlan(plan), "OR negative control must lose the composite index lookup")

		for _, original := range historyQueries {
			query := original
			innerLimit := regexp.MustCompile(`(?i)\s+LIMIT 1\s*\) rrep`)
			require.True(t, innerLimit.MatchString(query.sql))
			// The outer LIMIT remains, so results are still bounded but the
			// history work is not. Keep the subquery's ordering unchanged.
			query.sql = innerLimit.ReplaceAllString(query.sql, `) rrep`)
			plan := explainRepresentationQuery(t, db, query)
			require.Error(t, validateReporterHistoryPlan(plan), "outer-only limit must not pass the bounded-history validator")
		}
	})
	t.Logf("repository reads + EXPLAIN + negative controls: %s", time.Since(planStarted))
}

func explainRepresentationQuery(t *testing.T, db *gorm.DB, query representationPlanQuery) representationPlan {
	t.Helper()
	// database/sql preserves PostgreSQL's $n placeholders and avoids feeding
	// EXPLAIN back through the GORM callback used for repository SQL capture.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	var raw []byte
	require.NoError(t, sqlDB.QueryRowContext(t.Context(), "EXPLAIN (ANALYZE, FORMAT JSON) "+query.sql, query.vars...).Scan(&raw))
	var document []struct {
		Plan representationPlan `json:"Plan"`
	}
	require.NoError(t, json.Unmarshal(raw, &document))
	require.Len(t, document, 1)
	var logWork func(representationPlan)
	logWork = func(node representationPlan) {
		if node.RelationName != "" || strings.Contains(node.NodeType, "Sort") {
			t.Logf("%s %s: rows=%g loops=%g filtered=%g index=%q filter=%q",
				node.NodeType, node.RelationName, node.ActualRows, node.ActualLoops,
				node.RowsRemovedFilter, node.IndexCond, node.Filter)
		}
		for _, child := range node.Plans {
			logWork(child)
		}
	}
	logWork(document[0].Plan)
	return document[0].Plan
}

// TestRepresentationPlanValidatorsRejectUnboundedWork ensures one efficient
// subtree cannot conceal repeated probes or an expensive sibling scan.
func TestRepresentationPlanValidatorsRejectUnboundedWork(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		relation string
		index    string
		validate func(representationPlan) error
	}{
		{"common", "common_representations", "resource_id = rr.resource_id AND version = 1999", validateCommonRepresentationPlan},
		{"reporter", "reporter_representations", "reporter_resource_id = rr.id AND ROW(generation, version) < ROW(1, 999)", validateReporterHistoryPlan},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			scan := representationPlan{
				NodeType: "Index Scan", RelationName: scenario.relation,
				IndexCond: scenario.index, ActualRows: 1, ActualLoops: 1,
			}
			bounded := representationPlan{NodeType: "Limit", ActualRows: 1, Plans: []representationPlan{scan}}
			require.NoError(t, scenario.validate(bounded))

			repeated := scan
			repeated.ActualLoops = 100
			require.Error(t, scenario.validate(representationPlan{
				NodeType: "Limit", ActualRows: 1, Plans: []representationPlan{repeated},
			}))
			require.Error(t, scenario.validate(representationPlan{
				NodeType: "Append", Plans: []representationPlan{bounded, {
					NodeType: "Seq Scan", RelationName: scenario.relation, ActualRows: 100, ActualLoops: 1,
				}},
			}))
		})
	}
}

// representationScanWork counts total rows visited across every scan of a table.
// EXPLAIN reports per-loop averages, so looking at rows alone can hide repeated
// probes or an expensive sibling branch behind one efficient index lookup.
func representationScanWork(plan representationPlan, relation string) float64 {
	var work float64
	if plan.RelationName == relation {
		work = (plan.ActualRows + plan.RowsRemovedFilter) * plan.ActualLoops
	}
	for _, child := range plan.Plans {
		work += representationScanWork(child, relation)
	}
	return work
}

func validateCommonRepresentationPlan(plan representationPlan) error {
	if work := representationScanWork(plan, "common_representations"); work > 1 {
		return fmt.Errorf("common lookup visited %g rows, expected at most one: %+v", work, plan)
	}
	if plan.RelationName == "common_representations" &&
		strings.Contains(plan.NodeType, "Index") &&
		strings.Contains(plan.IndexCond, "resource_id =") && strings.Contains(plan.IndexCond, "version =") {
		return nil
	}
	for _, child := range plan.Plans {
		if validateCommonRepresentationPlan(child) == nil {
			return nil
		}
	}
	return fmt.Errorf("common lookup must index both resource_id and version: %+v", plan)
}

func validateReporterHistoryPlan(plan representationPlan) error {
	if work := representationScanWork(plan, "reporter_representations"); work > 4 {
		return fmt.Errorf("reporter history visited %g rows, expected at most one plus adjacent tombstones: %+v", work, plan)
	}
	// A Limit must bound an index lookup before any history-wide sort. Inspect
	// work as well as shape so a planner-flattened, equally efficient lookup is
	// accepted, but a top-N sort fed by all history is not.
	var boundedLookup func(representationPlan) bool
	boundedLookup = func(node representationPlan) bool {
		if node.RelationName == "reporter_representations" {
			return strings.Contains(node.NodeType, "Index") &&
				strings.Contains(node.IndexCond, "reporter_resource_id =") &&
				strings.Contains(node.IndexCond, "generation") && strings.Contains(node.IndexCond, "version") &&
				// One returned row plus the fixture's adjacent tombstones is
				// bounded work, regardless of the history's total length.
				node.ActualLoops > 0 && (node.ActualRows+node.RowsRemovedFilter)*node.ActualLoops <= 4
		}
		if strings.Contains(node.NodeType, "Sort") || node.NodeType == "Hash" {
			return false
		}
		for _, child := range node.Plans {
			if boundedLookup(child) {
				return true
			}
		}
		return false
	}
	if plan.NodeType == "Limit" && plan.ActualRows <= 1 && boundedLookup(plan) {
		return nil
	}
	for _, child := range plan.Plans {
		if validateReporterHistoryPlan(child) == nil {
			return nil
		}
	}
	return fmt.Errorf("reporter history needs a bounded index lookup without a history-wide sort: %+v", plan)
}
