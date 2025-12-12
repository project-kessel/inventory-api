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
	ResourceNamespace *string        `json:"resource_namespace,omitempty"`
	ResourceType      *string        `json:"resource_type,omitempty"`
	ResourceId        *string        `json:"resource_id,omitempty"`
	Relation          *string        `json:"relation,omitempty"`
	SubjectFilter     *SubjectFilter `json:"subject_filter,omitempty"`
}

// SubjectFilter for filtering subjects
type SubjectFilter struct {
	SubjectNamespace *string `json:"subject_namespace,omitempty"`
	SubjectType      *string `json:"subject_type,omitempty"`
	SubjectId        *string `json:"subject_id,omitempty"`
	Relation         *string `json:"relation,omitempty"`
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
