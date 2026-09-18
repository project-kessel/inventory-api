package data

import (
	"fmt"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// representationFetcher defines the interface for fetching representations from storage.
// This abstraction allows both the real repository (SQL) and fake repository (in-memory)
// to share the same control flow logic while using different storage backends.
type representationFetcher interface {
	fetchCommon(version uint) (bizmodel.Representation, *bizmodel.Version, error)
	fetchReporter(version uint, generation uint) (bizmodel.Representation, *bizmodel.Version, error)
	fetchPreviousReporter(currentVersion uint, currentGeneration uint) (bizmodel.Representation, *bizmodel.Version, error)
	fetchLastLiveReporterBefore(beforeVersion uint, beforeGeneration uint) (bizmodel.Representation, *bizmodel.Version, error)
}

// fetchCurrentAndPreviousRepresentations implements the common control flow for fetching
// current and previous representations from both common and reporter streams.
//
// This function embodies the two-stream model where:
// - If a version is provided (non-nil), that stream advanced - fetch current and previous
// - If a version is nil, that stream didn't advance - leave it nil (don't fetch)
func fetchCurrentAndPreviousRepresentations(
	fetcher representationFetcher,
	versions bizmodel.RepresentationVersions,
	operationType bizmodel.EventOperationType,
) (*bizmodel.Representations, *bizmodel.Representations, error) {
	currentCommonVersion := versions.CommonVersion()
	currentReporterVersion := versions.ReporterVersion()
	currentReporterGeneration := versions.ReporterGeneration()

	// Guard against both versions being nil (should not happen per TupleEvent invariant)
	if currentCommonVersion == nil && currentReporterVersion == nil {
		return nil, nil, fmt.Errorf("at least one version must be provided")
	}

	// Determine which streams advanced and fetch current/previous for each
	var currentCommon, previousCommon bizmodel.Representation
	var currentCommonVer, previousCommonVer *bizmodel.Version

	var currentReporter, previousReporter bizmodel.Representation
	var currentReporterVer, previousReporterVer *bizmodel.Version

	var err error

	// Handle delete operations specially - use upper-bound semantics to tolerate version gaps
	if operationType.OperationType() == bizmodel.OperationTypeDeleted {
		// For deletes, the event carries:
		// - commonVersion: unchanged during delete (already last-live)
		// - reporterVersion: tombstone version (incremented)
		// - reporterGeneration: unchanged
		//
		// We need to fetch the last-live state for tuple cleanup.

		// Common: exact fetch at the event's version (it's already last-live, unchanged during delete)
		if currentCommonVersion != nil {
			cv := currentCommonVersion.Uint()
			currentCommon, currentCommonVer, err = fetcher.fetchCommon(cv)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch current common representation: %w", err)
			}
		}

		// Reporter: use upper-bound fetch to find last-live before tombstone, tolerating gaps
		// The tombstone version itself is never returned (tombstone filter)
		// If tombstoneVersion is 0, the upper bound (generation=0, version < 0) is unsatisfiable,
		// so nothing is found - no special-casing needed.
		if currentReporterVersion != nil {
			rv := currentReporterVersion.Uint()
			rg := uint(0)
			if currentReporterGeneration != nil {
				rg = currentReporterGeneration.Uint()
			}
			// Fetch last-live reporter before the tombstone (gap-tolerant, tombstone-filtered)
			currentReporter, currentReporterVer, err = fetcher.fetchLastLiveReporterBefore(rv, rg)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch last live reporter before tombstone: %w", err)
			}
		}

		// For deletes, we only need the last-live state (in "current"), no "previous"
		// previous remains nil
	} else {
		// Non-delete operations: use exact fetch for current, upper-bound for previous

		// Fetch common stream - only if version is provided (stream advanced)
		if currentCommonVersion != nil {
			cv := currentCommonVersion.Uint()
			currentCommon, currentCommonVer, err = fetcher.fetchCommon(cv)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch current common representation: %w", err)
			}
			if operationType.OperationType() != bizmodel.OperationTypeCreated && cv > 0 {
				previousCommon, previousCommonVer, err = fetcher.fetchCommon(cv - 1)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to fetch previous common representation: %w", err)
				}
			}
		}
		// else: common stream didn't advance - leave nil (don't fetch)

		// Fetch reporter stream - only if version is provided (stream advanced)
		if currentReporterVersion != nil {
			rv := currentReporterVersion.Uint()
			rg := uint(0)
			if currentReporterGeneration != nil {
				rg = currentReporterGeneration.Uint()
			}
			currentReporter, currentReporterVer, err = fetcher.fetchReporter(rv, rg)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch current reporter representation: %w", err)
			}
			if operationType.OperationType() != bizmodel.OperationTypeCreated {
				// Fetch immediately preceding reporter row (by generation DESC, version DESC)
				// This handles both normal updates (previous version in same generation)
				// and revivals (tombstone in previous generation, even when current version is 0)
				previousReporter, previousReporterVer, err = fetcher.fetchPreviousReporter(rv, rg)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to fetch previous reporter representation: %w", err)
				}
			}
		}
		// else: reporter stream didn't advance - leave nil (don't fetch)
	}

	// Build current and previous Representations
	var current, previous *bizmodel.Representations

	// Only build current if at least one stream returned a version.
	// For tombstones (delete operations), the fetch returns (nil, nil, nil),
	// so both versions will be nil and we skip building current.
	if currentCommonVer != nil || currentReporterVer != nil {
		current, err = bizmodel.NewRepresentations(currentCommon, currentCommonVer, currentReporter, currentReporterVer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create current representation: %w", err)
		}
	}

	// Only build previous if at least one stream advanced to a previous version
	if previousCommonVer != nil || previousReporterVer != nil {
		previous, err = bizmodel.NewRepresentations(previousCommon, previousCommonVer, previousReporter, previousReporterVer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create previous representation: %w", err)
		}
	}

	return current, previous, nil
}
