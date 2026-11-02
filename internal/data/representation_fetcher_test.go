package data

import (
	"testing"

	"github.com/stretchr/testify/require"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// stubRepresentationFetcher satisfies representationFetcher without touching
// storage; fetchCurrentAndPreviousRepresentations is what is under test.
type stubRepresentationFetcher struct{}

func (stubRepresentationFetcher) fetchCommon(uint) (bizmodel.Representation, *bizmodel.Version, error) {
	return nil, nil, nil
}

func (stubRepresentationFetcher) fetchReporter(uint, uint) (bizmodel.Representation, *bizmodel.Version, error) {
	return nil, nil, nil
}

func (stubRepresentationFetcher) fetchPreviousReporter(uint, uint) (bizmodel.Representation, *bizmodel.Version, error) {
	return nil, nil, nil
}

func (stubRepresentationFetcher) fetchLastLiveReporterBefore(uint, uint) (bizmodel.Representation, *bizmodel.Version, error) {
	return nil, nil, nil
}

// TestFetchCurrentAndPreviousRepresentations_RejectsReporterVersionWithoutGeneration
// pins the legacy-event contract: a reporter version is only usable together with
// its generation, so a generation-less event must never be read as generation zero.
func TestFetchCurrentAndPreviousRepresentations_RejectsReporterVersionWithoutGeneration(t *testing.T) {
	for _, operationType := range []bizmodel.EventOperationType{
		bizmodel.OperationTypeCreated,
		bizmodel.OperationTypeUpdated,
		bizmodel.OperationTypeDeleted,
	} {
		t.Run(string(operationType.OperationType()), func(t *testing.T) {
			versions := bizmodel.NewRepresentationVersions(nil, ptrVersion(0), nil)

			_, _, err := fetchCurrentAndPreviousRepresentations(stubRepresentationFetcher{}, versions, operationType)

			require.Error(t, err)
			require.Contains(t, err.Error(), "without reporter generation")
		})
	}
}

// TestFetchCurrentAndPreviousRepresentations_AcceptsReporterCoordinates is the
// control: with the generation present the same event is accepted.
func TestFetchCurrentAndPreviousRepresentations_AcceptsReporterCoordinates(t *testing.T) {
	versions := bizmodel.NewRepresentationVersions(nil, ptrVersion(0), ptrGeneration(0))

	_, _, err := fetchCurrentAndPreviousRepresentations(stubRepresentationFetcher{}, versions, bizmodel.OperationTypeUpdated)

	require.NoError(t, err)
}

// TestInMemoryFetcher_FetchReporterReturnsVersionForTombstone pins the fake to the
// SQL repository's contract: a row that exists with empty data is a tombstone, so
// its version is returned alongside nil data. Only a missing row returns nil, nil.
func TestInMemoryFetcher_FetchReporterReturnsVersionForTombstone(t *testing.T) {
	fetcher := &inMemoryRepresentationFetcher{
		reporterReps: map[uint]map[uint]*storedReporterRepresentation{
			3: {0: {version: 3, generation: 0, tombstone: true}},
		},
	}

	data, version, err := fetcher.fetchReporter(3, 0)
	require.NoError(t, err)
	require.Nil(t, data)
	require.NotNil(t, version)
	require.Equal(t, uint(3), version.Uint())

	data, version, err = fetcher.fetchReporter(4, 0)
	require.NoError(t, err)
	require.Nil(t, data)
	require.Nil(t, version)
}
