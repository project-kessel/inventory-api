package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// TombstoneCleanupPolicy freezes the age cutoff and optional normalized reporter filter for one invocation.
type TombstoneCleanupPolicy struct {
	evaluationTime time.Time
	cutoff         time.Time
	reporterType   ReporterType
	hasReporter    bool
	valid          bool
}

// NewTombstoneCleanupPolicy constructs a positive-age policy using a frozen UTC evaluation time.
func NewTombstoneCleanupPolicy(evaluationTime time.Time, tombstoneDays int, reporterType string) (TombstoneCleanupPolicy, error) {
	if evaluationTime.IsZero() {
		return TombstoneCleanupPolicy{}, fmt.Errorf("tombstone cleanup requires a non-zero evaluation time")
	}
	if tombstoneDays <= 0 {
		return TombstoneCleanupPolicy{}, fmt.Errorf("tombstone age in days must be positive")
	}
	const maxDays = int64((1<<63 - 1) / int64(24*time.Hour))
	if int64(tombstoneDays) > maxDays {
		return TombstoneCleanupPolicy{}, fmt.Errorf("tombstone age in days is too large")
	}

	policy := TombstoneCleanupPolicy{
		evaluationTime: evaluationTime.UTC(),
		cutoff:         evaluationTime.UTC().Add(-time.Duration(tombstoneDays) * 24 * time.Hour),
		valid:          true,
	}
	if strings.TrimSpace(reporterType) != "" {
		validatedReporterType, err := NewReporterType(reporterType)
		if err != nil {
			return TombstoneCleanupPolicy{}, fmt.Errorf("invalid reporter type filter: %w", err)
		}
		policy.reporterType = validatedReporterType
		policy.hasReporter = true
	} else if reporterType != "" {
		return TombstoneCleanupPolicy{}, fmt.Errorf("reporter type filter cannot contain only whitespace")
	}
	return policy, nil
}

// Validate rejects the zero value, which is not a constructed cleanup policy.
func (policy TombstoneCleanupPolicy) Validate() error {
	if !policy.valid {
		return fmt.Errorf("tombstone cleanup policy must be constructed with NewTombstoneCleanupPolicy")
	}
	return nil
}

// EvaluationTime returns the invocation time frozen when the policy was constructed.
func (policy TombstoneCleanupPolicy) EvaluationTime() time.Time {
	return policy.evaluationTime
}

// Cutoff returns the frozen UTC cutoff; eligible tombstones must be strictly older than it.
func (policy TombstoneCleanupPolicy) Cutoff() time.Time {
	return policy.cutoff
}

// IsTombstoneEligible reports whether the tombstone timestamp is strictly before the frozen cutoff.
func (policy TombstoneCleanupPolicy) IsTombstoneEligible(updatedAt time.Time) bool {
	return policy.valid && !updatedAt.IsZero() && updatedAt.UTC().Before(policy.cutoff)
}

// ReporterType returns the normalized reporter filter and whether a filter was selected.
func (policy TombstoneCleanupPolicy) ReporterType() (ReporterType, bool) {
	return policy.reporterType, policy.hasReporter
}

// TombstoneCleanupImpact contains eligible or bounded row/entity/root impact counts.
type TombstoneCleanupImpact struct {
	reporterEntities        int64
	reporterResources       int64
	reporterRepresentations int64
	commonRepresentations   int64
	resourceRootsImpacted   int64
	resourcesRemovable      int64
	resourceRootIDs         map[ResourceId]struct{}
	rootIDsKnown            bool
}

// NewTombstoneCleanupImpact constructs non-negative cleanup impact counts.
func NewTombstoneCleanupImpact(
	reporterEntities int64,
	reporterResources int64,
	reporterRepresentations int64,
	commonRepresentations int64,
	resourceRootsImpacted int64,
	resourcesRemovable int64,
) (TombstoneCleanupImpact, error) {
	values := []int64{reporterEntities, reporterResources, reporterRepresentations, commonRepresentations, resourceRootsImpacted, resourcesRemovable}
	for _, value := range values {
		if value < 0 {
			return TombstoneCleanupImpact{}, fmt.Errorf("tombstone cleanup impact counts cannot be negative")
		}
	}
	if reporterEntities > reporterResources {
		return TombstoneCleanupImpact{}, fmt.Errorf("distinct reporter entities cannot exceed reporter metadata rows")
	}
	if resourcesRemovable > resourceRootsImpacted {
		return TombstoneCleanupImpact{}, fmt.Errorf("removable roots cannot exceed impacted roots")
	}
	return TombstoneCleanupImpact{
		reporterEntities:        reporterEntities,
		reporterResources:       reporterResources,
		reporterRepresentations: reporterRepresentations,
		commonRepresentations:   commonRepresentations,
		resourceRootsImpacted:   resourceRootsImpacted,
		resourcesRemovable:      resourcesRemovable,
		rootIDsKnown:            resourceRootsImpacted == 0,
	}, nil
}

// NewTombstoneCleanupImpactWithRootIDs constructs batch impact while retaining private root identity only for
// correct distinct-root aggregation. Root IDs are never exposed by the impact's public API.
func NewTombstoneCleanupImpactWithRootIDs(
	reporterEntities int64,
	reporterResources int64,
	reporterRepresentations int64,
	commonRepresentations int64,
	resourceRootIDs []ResourceId,
	resourcesRemovable int64,
) (TombstoneCleanupImpact, error) {
	uniqueRootIDs := make(map[ResourceId]struct{}, len(resourceRootIDs))
	for _, resourceRootID := range resourceRootIDs {
		if resourceRootID.UUID() == uuid.Nil {
			return TombstoneCleanupImpact{}, fmt.Errorf("tombstone cleanup root identities cannot be empty")
		}
		uniqueRootIDs[resourceRootID] = struct{}{}
	}
	impact, err := NewTombstoneCleanupImpact(
		reporterEntities,
		reporterResources,
		reporterRepresentations,
		commonRepresentations,
		int64(len(uniqueRootIDs)),
		resourcesRemovable,
	)
	if err != nil {
		return TombstoneCleanupImpact{}, err
	}
	impact.resourceRootIDs = uniqueRootIDs
	impact.rootIDsKnown = true
	return impact, nil
}

// ReporterEntities returns the number of distinct reporter entities impacted.
func (impact TombstoneCleanupImpact) ReporterEntities() int64 { return impact.reporterEntities }

// ReporterResources returns the number of reporter metadata rows proposed or removed.
func (impact TombstoneCleanupImpact) ReporterResources() int64 { return impact.reporterResources }

// ReporterRepresentations returns cascaded reporter-history rows proposed or removed.
func (impact TombstoneCleanupImpact) ReporterRepresentations() int64 {
	return impact.reporterRepresentations
}

// CommonRepresentations returns common-history rows proposed or removed with orphan roots.
func (impact TombstoneCleanupImpact) CommonRepresentations() int64 {
	return impact.commonRepresentations
}

// ResourceRootsImpacted returns distinct resource roots touched by the scope.
func (impact TombstoneCleanupImpact) ResourceRootsImpacted() int64 {
	return impact.resourceRootsImpacted
}

// ResourcesRemovable returns roots expected to be removed, or actually removed after execution.
func (impact TombstoneCleanupImpact) ResourcesRemovable() int64 { return impact.resourcesRemovable }

// Add combines committed batch impact, unioning private root identities so shared roots count once.
func (impact TombstoneCleanupImpact) Add(other TombstoneCleanupImpact) (TombstoneCleanupImpact, error) {
	values := [][2]int64{
		{impact.reporterEntities, other.reporterEntities},
		{impact.reporterResources, other.reporterResources},
		{impact.reporterRepresentations, other.reporterRepresentations},
		{impact.commonRepresentations, other.commonRepresentations},
		{impact.resourcesRemovable, other.resourcesRemovable},
	}
	for _, pair := range values {
		if pair[0] > int64(^uint64(0)>>1)-pair[1] {
			return TombstoneCleanupImpact{}, fmt.Errorf("tombstone cleanup impact count overflow")
		}
	}
	if (impact.resourceRootsImpacted > 0 && !impact.rootIDsKnown) ||
		(other.resourceRootsImpacted > 0 && !other.rootIDsKnown) {
		return TombstoneCleanupImpact{}, fmt.Errorf("cannot aggregate distinct tombstone cleanup roots without root identities")
	}
	resourceRootIDs := make(map[ResourceId]struct{}, len(impact.resourceRootIDs)+len(other.resourceRootIDs))
	for resourceRootID := range impact.resourceRootIDs {
		resourceRootIDs[resourceRootID] = struct{}{}
	}
	for resourceRootID := range other.resourceRootIDs {
		resourceRootIDs[resourceRootID] = struct{}{}
	}
	combined, err := NewTombstoneCleanupImpact(
		impact.reporterEntities+other.reporterEntities,
		impact.reporterResources+other.reporterResources,
		impact.reporterRepresentations+other.reporterRepresentations,
		impact.commonRepresentations+other.commonRepresentations,
		int64(len(resourceRootIDs)),
		impact.resourcesRemovable+other.resourcesRemovable,
	)
	if err != nil {
		return TombstoneCleanupImpact{}, err
	}
	combined.resourceRootIDs = resourceRootIDs
	combined.rootIDsKnown = true
	return combined, nil
}

// TombstoneCleanupExclusions counts age candidates protected for structural reasons.
type TombstoneCleanupExclusions struct {
	missingRoot                        int64
	rootTypeMismatch                   int64
	invalidReporterKey                 int64
	missingTerminalRepresentation      int64
	inconsistentTerminalRepresentation int64
	newerHistory                       int64
}

// NewTombstoneCleanupExclusions constructs non-negative, disjoint exclusion reason counts.
func NewTombstoneCleanupExclusions(missingRoot, rootTypeMismatch, invalidReporterKey, missingTerminalRepresentation, inconsistentTerminalRepresentation, newerHistory int64) (TombstoneCleanupExclusions, error) {
	values := []int64{missingRoot, rootTypeMismatch, invalidReporterKey, missingTerminalRepresentation, inconsistentTerminalRepresentation, newerHistory}
	for _, value := range values {
		if value < 0 {
			return TombstoneCleanupExclusions{}, fmt.Errorf("tombstone cleanup exclusion counts cannot be negative")
		}
	}
	if _, err := sumTombstoneCleanupExclusions(values); err != nil {
		return TombstoneCleanupExclusions{}, err
	}
	return TombstoneCleanupExclusions{
		missingRoot:                        missingRoot,
		rootTypeMismatch:                   rootTypeMismatch,
		invalidReporterKey:                 invalidReporterKey,
		missingTerminalRepresentation:      missingTerminalRepresentation,
		inconsistentTerminalRepresentation: inconsistentTerminalRepresentation,
		newerHistory:                       newerHistory,
	}, nil
}

// MissingRoot returns age candidates whose resource root is absent.
func (exclusions TombstoneCleanupExclusions) MissingRoot() int64 { return exclusions.missingRoot }

// RootTypeMismatch returns age candidates whose reporter and root types disagree.
func (exclusions TombstoneCleanupExclusions) RootTypeMismatch() int64 {
	return exclusions.rootTypeMismatch
}

// InvalidReporterKey returns age candidates with structurally invalid natural-key fields.
func (exclusions TombstoneCleanupExclusions) InvalidReporterKey() int64 {
	return exclusions.invalidReporterKey
}

// MissingTerminalRepresentation returns age candidates without their pointed terminal row.
func (exclusions TombstoneCleanupExclusions) MissingTerminalRepresentation() int64 {
	return exclusions.missingTerminalRepresentation
}

// InconsistentTerminalRepresentation returns age candidates whose terminal row violates tombstone shape.
func (exclusions TombstoneCleanupExclusions) InconsistentTerminalRepresentation() int64 {
	return exclusions.inconsistentTerminalRepresentation
}

// NewerHistory returns age candidates with history newer than the authoritative reporter pointer.
func (exclusions TombstoneCleanupExclusions) NewerHistory() int64 { return exclusions.newerHistory }

// Total returns the number of age candidates excluded for a structural reason.
func (exclusions TombstoneCleanupExclusions) Total() int64 {
	total, _ := sumTombstoneCleanupExclusions([]int64{
		exclusions.missingRoot,
		exclusions.rootTypeMismatch,
		exclusions.invalidReporterKey,
		exclusions.missingTerminalRepresentation,
		exclusions.inconsistentTerminalRepresentation,
		exclusions.newerHistory,
	})
	return total
}

// TombstoneCleanupReport separates full eligibility from the configured bounded execution projection.
type TombstoneCleanupReport struct {
	evaluatedAt          time.Time
	snapshotAt           time.Time
	cutoff               time.Time
	boundedRowLimit      int64
	ageCandidates        int64
	unknownTimestampRows int64
	totalEligible        TombstoneCleanupImpact
	boundedImpact        TombstoneCleanupImpact
	exclusions           TombstoneCleanupExclusions
}

// NewTombstoneCleanupReport validates a read-only report and its candidate accounting.
func NewTombstoneCleanupReport(
	evaluatedAt time.Time,
	snapshotAt time.Time,
	cutoff time.Time,
	boundedRowLimit int64,
	ageCandidates int64,
	unknownTimestampRows int64,
	totalEligible TombstoneCleanupImpact,
	boundedImpact TombstoneCleanupImpact,
	exclusions TombstoneCleanupExclusions,
) (TombstoneCleanupReport, error) {
	if evaluatedAt.IsZero() || snapshotAt.IsZero() || cutoff.IsZero() || !cutoff.Before(evaluatedAt) {
		return TombstoneCleanupReport{}, fmt.Errorf("tombstone cleanup report requires valid evaluation, snapshot, and cutoff times")
	}
	if boundedRowLimit < 0 || ageCandidates < 0 || unknownTimestampRows < 0 {
		return TombstoneCleanupReport{}, fmt.Errorf("tombstone cleanup report limits and candidates cannot be negative")
	}
	if boundedImpact.reporterResources > boundedRowLimit {
		return TombstoneCleanupReport{}, fmt.Errorf("bounded reporter rows exceed configured row limit")
	}
	if !impactFitsWithin(boundedImpact, totalEligible) {
		return TombstoneCleanupReport{}, fmt.Errorf("bounded tombstone impact cannot exceed total eligibility")
	}
	eligibleAndExcluded, err := safeTombstoneCountAdd(totalEligible.reporterResources, exclusions.Total())
	if err != nil || ageCandidates != eligibleAndExcluded {
		return TombstoneCleanupReport{}, fmt.Errorf("age candidates must equal eligible reporter rows plus structural exclusions")
	}
	return TombstoneCleanupReport{
		evaluatedAt:          evaluatedAt.UTC(),
		snapshotAt:           snapshotAt.UTC(),
		cutoff:               cutoff.UTC(),
		boundedRowLimit:      boundedRowLimit,
		ageCandidates:        ageCandidates,
		unknownTimestampRows: unknownTimestampRows,
		totalEligible:        totalEligible,
		boundedImpact:        boundedImpact,
		exclusions:           exclusions,
	}, nil
}

// EvaluatedAt returns the UTC time at which the invocation policy was frozen.
func (report TombstoneCleanupReport) EvaluatedAt() time.Time { return report.evaluatedAt }

// SnapshotAt returns the UTC snapshot timestamp used by the read-only database report.
func (report TombstoneCleanupReport) SnapshotAt() time.Time { return report.snapshotAt }

// Cutoff returns the frozen strict UTC trial cutoff.
func (report TombstoneCleanupReport) Cutoff() time.Time { return report.cutoff }

// BoundedRowLimit returns the configured reporter-row budget used for the bounded projection.
func (report TombstoneCleanupReport) BoundedRowLimit() int64 { return report.boundedRowLimit }

// AgeCandidates returns tombstoned reporter rows matching only the age/filter selector.
func (report TombstoneCleanupReport) AgeCandidates() int64 { return report.ageCandidates }

// UnknownTimestampRows returns tombstoned metadata withheld because updated_at is NULL.
func (report TombstoneCleanupReport) UnknownTimestampRows() int64 { return report.unknownTimestampRows }

// TotalEligible returns structurally valid impact across the full policy scope.
func (report TombstoneCleanupReport) TotalEligible() TombstoneCleanupImpact {
	return report.totalEligible
}

// BoundedImpact returns projected impact for the deterministic configured row budget.
func (report TombstoneCleanupReport) BoundedImpact() TombstoneCleanupImpact {
	return report.boundedImpact
}

// Exclusions returns age candidates withheld for structural safety reasons.
func (report TombstoneCleanupReport) Exclusions() TombstoneCleanupExclusions {
	return report.exclusions
}

func impactFitsWithin(bounded, total TombstoneCleanupImpact) bool {
	return bounded.reporterEntities <= total.reporterEntities &&
		bounded.reporterResources <= total.reporterResources &&
		bounded.reporterRepresentations <= total.reporterRepresentations &&
		bounded.commonRepresentations <= total.commonRepresentations &&
		bounded.resourceRootsImpacted <= total.resourceRootsImpacted &&
		bounded.resourcesRemovable <= total.resourcesRemovable
}

func sumTombstoneCleanupExclusions(values []int64) (int64, error) {
	var total int64
	for _, value := range values {
		if value < 0 {
			return 0, fmt.Errorf("tombstone cleanup exclusion counts cannot be negative")
		}
		var err error
		total, err = safeTombstoneCountAdd(total, value)
		if err != nil {
			return 0, err
		}
	}
	return total, nil
}

func safeTombstoneCountAdd(left, right int64) (int64, error) {
	maxInt64 := int64(^uint64(0) >> 1)
	if left < 0 || right < 0 || left > maxInt64-right {
		return 0, fmt.Errorf("tombstone cleanup count overflow or negative count")
	}
	return left + right, nil
}
