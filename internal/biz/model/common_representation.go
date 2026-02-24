package model

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type CommonRepresentation struct {
	Representation
	resourceId    ResourceId
	version       Version
	reporter      ReporterId
	transactionId TransactionId
}

func NewCommonRepresentation(
	resourceId ResourceId,
	data Representation,
	version Version,
	reporterType ReporterType,
	reporterInstanceId ReporterInstanceId,
	transactionId TransactionId,
) (CommonRepresentation, error) {
	if resourceId.UUID() == uuid.Nil {
		return CommonRepresentation{}, fmt.Errorf("%w: ResourceId", ErrInvalidUUID)
	}

	if strings.TrimSpace(string(reporterType)) == "" {
		return CommonRepresentation{}, fmt.Errorf("%w: ReporterType", ErrEmpty)
	}

	if strings.TrimSpace(string(reporterInstanceId)) == "" {
		return CommonRepresentation{}, fmt.Errorf("%w: ReporterInstanceId", ErrEmpty)
	}
	if len(data) == 0 {
		return CommonRepresentation{}, fmt.Errorf("%w: CommonRepresentation data", ErrInvalidData)
	}

	if data.Data() == nil {
		return CommonRepresentation{}, fmt.Errorf("CommonRepresentation requires non-empty data")
	}

	reporter := NewReporterId(reporterType, reporterInstanceId)

	return CommonRepresentation{
		Representation: data,
		resourceId:     resourceId,
		version:        version,
		reporter:       reporter,
		transactionId:  transactionId,
	}, nil
}

func (cr CommonRepresentation) Serialize() CommonRepresentationSnapshot {
	reporterType, reporterInstanceId := cr.reporter.Serialize()

	// Create representation snapshot
	representationSnapshot := RepresentationSnapshot{
		Data: cr.Data(),
	}

	// TransactionId: nil when empty (optional), else pointer to serialized value
	var txID *string
	if s := cr.transactionId.Serialize(); s != "" {
		txID = &s
	}
	now := time.Now()
	return CommonRepresentationSnapshot{
		Representation:             representationSnapshot,
		ResourceId:                 cr.resourceId.UUID(),
		Version:                    cr.version.Serialize(),
		ReportedByReporterType:     reporterType,
		ReportedByReporterInstance: reporterInstanceId,
		TransactionId:              txID,
		CreatedAt:                  now,
	}
}

func DeserializeCommonRepresentation(snapshot *CommonRepresentationSnapshot) CommonRepresentation {
	// Create domain tiny types directly from snapshot values - no validation
	resourceId := ResourceId(snapshot.ResourceId)
	representation := Representation(snapshot.Representation.Data)
	version := DeserializeVersion(snapshot.Version)
	reporterType := ReporterType(snapshot.ReportedByReporterType)
	reporterInstanceId := ReporterInstanceId(snapshot.ReportedByReporterInstance)
	txIDStr := ""
	if snapshot.TransactionId != nil {
		txIDStr = *snapshot.TransactionId
	}
	transactionId := TransactionId(txIDStr)

	// Create reporter ID
	reporterId := ReporterId{
		reporterType:       reporterType,
		reporterInstanceId: reporterInstanceId,
	}

	return CommonRepresentation{
		Representation: representation,
		resourceId:     resourceId,
		version:        version,
		reporter:       reporterId,
		transactionId:  transactionId,
	}
}

// CreateSnapshot creates a snapshot of the CommonRepresentation
func (cr CommonRepresentation) CreateSnapshot() (CommonRepresentationSnapshot, error) {
	return cr.Serialize(), nil
}
