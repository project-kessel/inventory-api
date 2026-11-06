package relationships

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
)

// AddResourceRelationshipsCommand contains the data needed to add one or more
// relationships to a resource. This is the domain command used by the usecase
// layer, decoupled from protobuf types.
type AddResourceRelationshipsCommand struct {
	// Resource identification
	LocalResourceId    model.LocalResourceId
	ResourceType       model.ResourceType
	ReporterType       model.ReporterType
	ReporterInstanceId model.ReporterInstanceId

	// Idempotency key. When nil (or empty) a new transaction ID is generated.
	TransactionId *model.TransactionId

	// Partial representations containing only relationship-typed fields.
	// The values are merged additively into the stored representations.
	// At least one must be provided.
	ReporterRepresentation *model.Representation
	CommonRepresentation   *model.Representation

	// Write behavior
	WriteVisibility resources.WriteVisibility
}
