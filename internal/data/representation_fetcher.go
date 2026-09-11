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
	fetchReporter(version uint) (bizmodel.Representation, *bizmodel.Version, error)
	fetchPreviousReporter(currentVersion uint) (bizmodel.Representation, *bizmodel.Version, error)
}

// fetchCurrentAndPreviousRepresentations implements the common control flow for fetching
// current and previous representations from both common and reporter streams.
//
// This function embodies the two-stream model where:
// - If a version is provided (non-nil), that stream advanced - fetch current and previous
// - If a version is nil, that stream didn't advance - leave it nil (don't fetch)
func fetchCurrentAndPreviousRepresentations(
	fetcher representationFetcher,
	currentCommonVersion *bizmodel.Version,
	currentReporterVersion *bizmodel.Version,
	operationType bizmodel.EventOperationType,
) (*bizmodel.Representations, *bizmodel.Representations, error) {
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
		currentReporter, currentReporterVer, err = fetcher.fetchReporter(rv)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch current reporter representation: %w", err)
		}
		if operationType.OperationType() != bizmodel.OperationTypeCreated {
			// Fetch immediately preceding reporter row (by generation DESC, version DESC)
			// This handles both normal updates (previous version in same generation)
			// and revivals (tombstone in previous generation, even when current version is 0)
			previousReporter, previousReporterVer, err = fetcher.fetchPreviousReporter(rv)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to fetch previous reporter representation: %w", err)
			}
		}
	}
	// else: reporter stream didn't advance - leave nil (don't fetch)

	// Build current and previous Representations
	var current, previous *bizmodel.Representations

	current, err = bizmodel.NewRepresentations(currentCommon, currentCommonVer, currentReporter, currentReporterVer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create current representation: %w", err)
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
