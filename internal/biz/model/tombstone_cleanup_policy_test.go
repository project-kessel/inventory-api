package model

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTombstoneCleanupPolicyFreezesUTCAndUsesStrictCutoff(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 12, 30, 0, 0, time.FixedZone("trial", -5*60*60))
	policy, err := NewTombstoneCleanupPolicy(startedAt, 7, " HBI ")
	require.NoError(t, err)

	expectedRunAt := startedAt.UTC()
	expectedCutoff := expectedRunAt.Add(-168 * time.Hour)
	assert.Equal(t, expectedRunAt, policy.EvaluationTime())
	assert.Equal(t, expectedCutoff, policy.Cutoff())
	assert.True(t, policy.IsTombstoneEligible(expectedCutoff.Add(-time.Nanosecond)))
	assert.False(t, policy.IsTombstoneEligible(expectedCutoff), "equality is retained")
	assert.False(t, policy.IsTombstoneEligible(expectedCutoff.Add(time.Nanosecond)))
	assert.False(t, policy.IsTombstoneEligible(time.Time{}), "an absent or untrusted timestamp is not an age candidate")

	reporterType, filtered := policy.ReporterType()
	assert.True(t, filtered)
	assert.Equal(t, "hbi", reporterType.String())
}

func TestNewTombstoneCleanupPolicyAcceptsUnfilteredScope(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	policy, err := NewTombstoneCleanupPolicy(startedAt, 1, "")
	require.NoError(t, err)

	_, filtered := policy.ReporterType()
	assert.False(t, filtered)
}

func TestNewTombstoneCleanupPolicyRejectsInvalidInputs(t *testing.T) {
	startedAt := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	for _, days := range []int{0, -1, math.MaxInt} {
		_, err := NewTombstoneCleanupPolicy(startedAt, days, "")
		assert.Error(t, err, "days=%d", days)
	}
	_, err := NewTombstoneCleanupPolicy(time.Time{}, 1, "")
	assert.Error(t, err)
	_, err = NewTombstoneCleanupPolicy(startedAt, 1, "   ")
	assert.Error(t, err)
	assert.Error(t, (TombstoneCleanupPolicy{}).Validate())
}

func TestTombstoneCleanupImpactAddsAffectedRowsWithoutOverflow(t *testing.T) {
	firstRootIDs := []ResourceId{newCleanupResourceID(t), newCleanupResourceID(t)}
	secondRootIDs := []ResourceId{newCleanupResourceID(t), newCleanupResourceID(t)}
	first, err := NewTombstoneCleanupImpactWithRootIDs(3, 3, 8, 4, firstRootIDs, 1)
	require.NoError(t, err)
	second, err := NewTombstoneCleanupImpactWithRootIDs(2, 2, 5, 3, secondRootIDs, 1)
	require.NoError(t, err)
	got, err := first.Add(second)
	require.NoError(t, err)
	want, err := NewTombstoneCleanupImpactWithRootIDs(5, 5, 13, 7, append(firstRootIDs, secondRootIDs...), 2)
	require.NoError(t, err)
	assert.Equal(t, want, got)

	_, err = NewTombstoneCleanupImpact(0, 0, -1, 0, 0, 0)
	assert.Error(t, err)
	maximum, err := NewTombstoneCleanupImpact(0, 0, math.MaxInt64, 0, 0, 0)
	require.NoError(t, err)
	_, err = maximum.Add(first)
	assert.Error(t, err)
}

func newCleanupResourceID(t *testing.T) ResourceId {
	t.Helper()
	resourceID, err := NewResourceId(uuid.New())
	require.NoError(t, err)
	return resourceID
}

func TestTombstoneCleanupImpactCountsDistinctRootsAcrossBatches(t *testing.T) {
	sharedRoot, err := NewResourceId(uuid.New())
	require.NoError(t, err)
	secondRoot, err := NewResourceId(uuid.New())
	require.NoError(t, err)

	first, err := NewTombstoneCleanupImpactWithRootIDs(1, 1, 2, 0, []ResourceId{sharedRoot}, 0)
	require.NoError(t, err)
	second, err := NewTombstoneCleanupImpactWithRootIDs(2, 2, 3, 0, []ResourceId{sharedRoot, secondRoot}, 1)
	require.NoError(t, err)

	combined, err := first.Add(second)
	require.NoError(t, err)
	assert.Equal(t, int64(3), combined.ReporterEntities())
	assert.Equal(t, int64(3), combined.ReporterResources())
	assert.Equal(t, int64(5), combined.ReporterRepresentations())
	assert.Equal(t, int64(2), combined.ResourceRootsImpacted(), "the shared root must be counted once")
	assert.Equal(t, int64(1), combined.ResourcesRemovable())
}

func TestTombstoneCleanupReportSeparatesEligibleAndBoundedImpact(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	cutoff := evaluatedAt.Add(-168 * time.Hour)
	total, err := NewTombstoneCleanupImpact(5, 5, 17, 9, 4, 2)
	require.NoError(t, err)
	bounded, err := NewTombstoneCleanupImpact(2, 2, 6, 3, 2, 1)
	require.NoError(t, err)
	exclusions, err := NewTombstoneCleanupExclusions(1, 0, 0, 1, 0, 0)
	require.NoError(t, err)
	snapshotAt := evaluatedAt.Add(2 * time.Second)
	report, err := NewTombstoneCleanupReport(evaluatedAt, snapshotAt, cutoff, 2, 7, 0, total, bounded, exclusions)
	require.NoError(t, err)

	assert.Equal(t, evaluatedAt, report.EvaluatedAt())
	assert.Equal(t, snapshotAt, report.SnapshotAt())
	assert.Equal(t, cutoff, report.Cutoff())
	assert.Equal(t, int64(2), report.BoundedRowLimit())
	assert.Equal(t, int64(7), report.AgeCandidates())
	assert.Zero(t, report.UnknownTimestampRows())
	assert.Equal(t, total, report.TotalEligible())
	assert.Equal(t, bounded, report.BoundedImpact())
	assert.Equal(t, exclusions, report.Exclusions())
}

func TestTombstoneCleanupReportRejectsInconsistentTotals(t *testing.T) {
	evaluatedAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	cutoff := evaluatedAt.Add(-168 * time.Hour)
	total, err := NewTombstoneCleanupImpact(2, 2, 4, 1, 2, 1)
	require.NoError(t, err)
	bounded, err := NewTombstoneCleanupImpact(3, 3, 4, 1, 2, 1)
	require.NoError(t, err)
	exclusions, err := NewTombstoneCleanupExclusions(0, 0, 0, 0, 0, 0)
	require.NoError(t, err)
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 2, 0, total, bounded, exclusions)
	assert.Error(t, err, "bounded impact cannot exceed total eligibility")
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 3, 0, total, total, exclusions)
	assert.Error(t, err, "age candidates must reconcile to eligible plus protected rows")
	_, err = NewTombstoneCleanupReport(evaluatedAt, evaluatedAt, cutoff, 2, 2, -1, total, total, exclusions)
	assert.Error(t, err, "unknown timestamp count cannot be negative")
}
