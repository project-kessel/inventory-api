package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
)

// ReporterRepresentation captures the **reporter-specific view** of a resource.  Each reporter maintains
// its own version & generation counters which evolve independently of the `CommonRepresentation`.  The
// struct purposefully embeds `Representation` so the JSON `Data` blob remains first-class.
type ReporterRepresentation struct {
	Representation

	ReporterResourceID uuid.UUID
	Version            uint
	Generation         uint
	ReporterVersion    *string
	CommonVersion      uint
	TransactionId      string
	Tombstone          bool
	CreatedAt          time.Time

	ReporterResource ReporterResource
}

// NewReporterRepresentation is the ONLY factory for creating a ReporterRepresentation. It guarantees that
// every instance is fully validated (IDs present, counters non-negative, optional fields length-checked)
// before it enters the system.
func NewReporterRepresentation(
	data internal.JsonObject,
	reporterResourceID uuid.UUID,
	version uint,
	generation uint,
	commonVersion uint,
	transactionId string,
	tombstone bool,
	reporterVersion *string,
) (*ReporterRepresentation, error) {
	rr := &ReporterRepresentation{
		Representation: Representation{
			Data: data,
		},
		ReporterResourceID: reporterResourceID,
		Version:            version,
		Generation:         generation,
		CommonVersion:      commonVersion,
		TransactionId:      transactionId,
		Tombstone:          tombstone,
		ReporterVersion:    reporterVersion,
	}

	if err := validateReporterRepresentation(rr); err != nil {
		return nil, err
	}

	return rr, nil
}

func validateReporterRepresentation(rr *ReporterRepresentation) error {
	return bizmodel.AggregateErrors(
		bizmodel.ValidateUUIDRequired("ReporterResourceID", rr.ReporterResourceID),
		bizmodel.ValidateMinValueUint("Version", rr.Version, MinVersionValue),
		bizmodel.ValidateMinValueUint("Generation", rr.Generation, MinGenerationValue),
		bizmodel.ValidateMinValueUint("CommonVersion", rr.CommonVersion, MinCommonVersion),
		bizmodel.ValidateMaxLength("TransactionId", rr.TransactionId, MaxTransactionIdLength),
		bizmodel.ValidateOptionalString("ReporterVersion", rr.ReporterVersion, MaxReporterVersionLength),
	)
}

// SerializeToSnapshot converts GORM ReporterRepresentation to snapshot type - direct initialization without validation
func (rr ReporterRepresentation) SerializeToSnapshot() bizmodel.ReporterRepresentationSnapshot {
	// Create representation snapshot
	representationSnapshot := bizmodel.RepresentationSnapshot{
		Data: rr.Data,
	}

	return bizmodel.ReporterRepresentationSnapshot{
		Representation:     representationSnapshot,
		ReporterResourceID: rr.ReporterResourceID,
		Version:            rr.Version,
		Generation:         rr.Generation,
		ReporterVersion:    rr.ReporterVersion,
		CommonVersion:      rr.CommonVersion,
		TransactionId:      rr.TransactionId,
		Tombstone:          rr.Tombstone,
		CreatedAt:          rr.CreatedAt,
	}
}

// DeserializeReporterRepresentationFromSnapshot creates GORM ReporterRepresentation from snapshot - direct initialization without validation
func DeserializeReporterRepresentationFromSnapshot(snapshot bizmodel.ReporterRepresentationSnapshot) ReporterRepresentation {
	return ReporterRepresentation{
		Representation: Representation{
			Data: snapshot.Representation.Data,
		},
		ReporterResourceID: snapshot.ReporterResourceID,
		Version:            snapshot.Version,
		Generation:         snapshot.Generation,
		ReporterVersion:    snapshot.ReporterVersion,
		CommonVersion:      snapshot.CommonVersion,
		TransactionId:      snapshot.TransactionId,
		Tombstone:          snapshot.Tombstone,
		CreatedAt:          snapshot.CreatedAt,
	}
}
