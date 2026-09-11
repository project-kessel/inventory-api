package data

import (
	"fmt"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// representationFetcher defines the interface for fetching representations from storage.
// This abstraction allows both the real repository (SQL) and fake repository (in-memory)
// to share the same control flow logic while using different storage backends.
type representationFetcher interface {
	fetchCommon(version uint) (bizmodel.Representation, *bizmodel.Version)
	fetchReporter(version uint) (bizmodel.Representation, *bizmodel.Version)
	fetchPreviousReporter(currentVersion uint) (bizmodel.Representation, *bizmodel.Version)
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

	// Fetch common stream - only if version is provided (stream advanced)
	if currentCommonVersion != nil {
		cv := currentCommonVersion.Uint()
		currentCommon, currentCommonVer = fetcher.fetchCommon(cv)
		if operationType.OperationType() != bizmodel.OperationTypeCreated && cv > 0 {
			previousCommon, previousCommonVer = fetcher.fetchCommon(cv - 1)
		}
	}
	// else: common stream didn't advance - leave nil (don't fetch)

	// Fetch reporter stream - only if version is provided (stream advanced)
	if currentReporterVersion != nil {
		rv := currentReporterVersion.Uint()
		currentReporter, currentReporterVer = fetcher.fetchReporter(rv)
		if operationType.OperationType() != bizmodel.OperationTypeCreated && rv > 0 {
			// Fetch immediately preceding reporter row (by version DESC, generation DESC)
			previousReporter, previousReporterVer = fetcher.fetchPreviousReporter(rv)
		}
	}
	// else: reporter stream didn't advance - leave nil (don't fetch)

	// Build current and previous Representations
	var current, previous *bizmodel.Representations
	var err error

	current, err = bizmodel.NewRepresentations(currentCommon, currentCommonVer, currentReporter, currentReporterVer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create current representation: %w", err)
	}

	// Only build previous if at least one stream has data
	if len(previousCommon) > 0 || len(previousReporter) > 0 {
		previous, err = bizmodel.NewRepresentations(previousCommon, previousCommonVer, previousReporter, previousReporterVer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create previous representation: %w", err)
		}
	}

	return current, previous, nil
}
