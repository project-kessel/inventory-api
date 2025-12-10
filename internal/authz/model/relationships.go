package model

// CreateRelationshipsResponse returned from creating relationships
type CreateRelationshipsResponse struct {
	ConsistencyToken *ConsistencyToken
}

// DeleteRelationshipsResponse returned from deleting relationships
type DeleteRelationshipsResponse struct {
	ConsistencyToken *ConsistencyToken
}

// RelationTupleFilter for filtering relationships
type RelationTupleFilter struct {
	ResourceNamespace *string
	ResourceType      *string
	ResourceId        *string
	Relation          *string
	SubjectFilter     *SubjectFilter
}

// SubjectFilter for filtering subjects
type SubjectFilter struct {
	SubjectNamespace *string
	SubjectType      *string
	SubjectId        *string
	Relation         *string
}

// FencingCheck for distributed lock validation
type FencingCheck struct {
	LockId    string
	LockToken string
}

// AcquireLockResponse returned from lock acquisition
type AcquireLockResponse struct {
	LockToken string
}

// ImportBulkTuplesRequest for bulk import (streaming)
type ImportBulkTuplesRequest struct {
	Tuples []*Relationship
}

// ImportBulkTuplesResponse from bulk import
type ImportBulkTuplesResponse struct {
	NumImported uint64
}
