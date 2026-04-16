package tuples

import (
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// CreateTuplesCommand - domain command for creating tuples (DEPRECATED).
// This command exists only for RBAC backward compatibility and will be removed.
type CreateTuplesCommand struct {
	Tuples       []model.RelationsTuple
	Upsert       bool
	FencingCheck *FencingCheck
}

// DeleteTuplesCommand - domain command for deleting tuples (DEPRECATED).
// This command exists only for RBAC backward compatibility and will be removed.
type DeleteTuplesCommand struct {
	Filter       TupleFilter
	FencingCheck *FencingCheck
}

// ReadTuplesCommand - domain command for reading tuples (DEPRECATED).
// This command exists only for RBAC backward compatibility and will be removed.
type ReadTuplesCommand struct {
	Filter      TupleFilter
	Pagination  *model.Pagination
	Consistency model.Consistency
}

// AcquireLockCommand - domain command for acquiring locks (DEPRECATED).
// This command exists only for RBAC backward compatibility and will be removed.
type AcquireLockCommand struct {
	LockId string
}

// TupleFilter represents filtering criteria for tuple queries.
type TupleFilter struct {
	ResourceNamespace *string
	ResourceType      *string
	ResourceId        *string
	Relation          *string
	SubjectFilter     *SubjectFilter
}

// SubjectFilter represents subject filtering criteria.
type SubjectFilter struct {
	SubjectNamespace *string
	SubjectType      *string
	SubjectId        *string
	Relation         *string
}

// FencingCheck represents distributed locking parameters.
type FencingCheck struct {
	LockId    string
	LockToken string
}

// CreateTuplesResult - result for CreateTuples operation.
type CreateTuplesResult struct {
	ConsistencyToken model.ConsistencyToken
}

// DeleteTuplesResult - result for DeleteTuples operation.
type DeleteTuplesResult struct {
	ConsistencyToken model.ConsistencyToken
}

// AcquireLockResult - result for AcquireLock operation.
type AcquireLockResult struct {
	LockToken string
}
