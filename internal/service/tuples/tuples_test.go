package tuples

import (
	"testing"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	relationspb "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"github.com/stretchr/testify/assert"
)

// TestRelationshipToV1beta1 tests conversion from inventory relationship to relations-api format.
func TestRelationshipToV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.Relationship
		expected *relationspb.Relationship
	}{
		{
			name: "valid relationship",
			input: &pb.Relationship{
				Resource: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "workspace",
					},
					Id: "ws-1",
				},
				Relation: "member",
				Subject: &pb.RelationSubjectReference{
					Subject: &pb.RelationObjectReference{
						Type: &pb.RelationObjectType{
							Namespace: "rbac",
							Name:      "principal",
						},
						Id: "user-1",
					},
				},
			},
			expected: &relationspb.Relationship{
				Resource: &relationspb.ObjectReference{
					Type: &relationspb.ObjectType{
						Namespace: "rbac",
						Name:      "workspace",
					},
					Id: "ws-1",
				},
				Relation: "member",
				Subject: &relationspb.SubjectReference{
					Subject: &relationspb.ObjectReference{
						Type: &relationspb.ObjectType{
							Namespace: "rbac",
							Name:      "principal",
						},
						Id: "user-1",
					},
				},
			},
		},
		{
			name:     "nil relationship",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relationshipToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestRelationshipFromV1beta1 tests conversion from relations-api format back to inventory.
func TestRelationshipFromV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    *relationspb.Relationship
		expected *pb.Relationship
	}{
		{
			name: "valid relationship",
			input: &relationspb.Relationship{
				Resource: &relationspb.ObjectReference{
					Type: &relationspb.ObjectType{
						Namespace: "rbac",
						Name:      "workspace",
					},
					Id: "ws-1",
				},
				Relation: "member",
				Subject: &relationspb.SubjectReference{
					Subject: &relationspb.ObjectReference{
						Type: &relationspb.ObjectType{
							Namespace: "rbac",
							Name:      "principal",
						},
						Id: "user-1",
					},
				},
			},
			expected: &pb.Relationship{
				Resource: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "workspace",
					},
					Id: "ws-1",
				},
				Relation: "member",
				Subject: &pb.RelationSubjectReference{
					Subject: &pb.RelationObjectReference{
						Type: &pb.RelationObjectType{
							Namespace: "rbac",
							Name:      "principal",
						},
						Id: "user-1",
					},
				},
			},
		},
		{
			name:     "nil relationship",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := relationshipFromV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestObjectReferenceToV1beta1 tests conversion of object references.
func TestObjectReferenceToV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.RelationObjectReference
		expected *relationspb.ObjectReference
	}{
		{
			name: "valid object reference",
			input: &pb.RelationObjectReference{
				Type: &pb.RelationObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "ws-1",
			},
			expected: &relationspb.ObjectReference{
				Type: &relationspb.ObjectType{
					Namespace: "rbac",
					Name:      "workspace",
				},
				Id: "ws-1",
			},
		},
		{
			name:     "nil object reference",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := objectReferenceToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSubjectReferenceToV1beta1 tests conversion of subject references.
func TestSubjectReferenceToV1beta1(t *testing.T) {
	relation := "members"
	tests := []struct {
		name     string
		input    *pb.RelationSubjectReference
		expected *relationspb.SubjectReference
	}{
		{
			name: "subject reference with relation",
			input: &pb.RelationSubjectReference{
				Relation: &relation,
				Subject: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "group",
					},
					Id: "group-1",
				},
			},
			expected: &relationspb.SubjectReference{
				Relation: &relation,
				Subject: &relationspb.ObjectReference{
					Type: &relationspb.ObjectType{
						Namespace: "rbac",
						Name:      "group",
					},
					Id: "group-1",
				},
			},
		},
		{
			name: "subject reference without relation",
			input: &pb.RelationSubjectReference{
				Subject: &pb.RelationObjectReference{
					Type: &pb.RelationObjectType{
						Namespace: "rbac",
						Name:      "principal",
					},
					Id: "user-1",
				},
			},
			expected: &relationspb.SubjectReference{
				Subject: &relationspb.ObjectReference{
					Type: &relationspb.ObjectType{
						Namespace: "rbac",
						Name:      "principal",
					},
					Id: "user-1",
				},
			},
		},
		{
			name:     "nil subject reference",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := subjectReferenceToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTupleFilterToV1beta1 tests conversion of tuple filters.
func TestTupleFilterToV1beta1(t *testing.T) {
	namespace := "rbac"
	resourceType := "workspace"
	resourceID := "ws-1"
	relation := "member"

	tests := []struct {
		name     string
		input    *pb.RelationTupleFilter
		expected *relationspb.RelationTupleFilter
	}{
		{
			name: "filter with all fields",
			input: &pb.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
				ResourceId:        &resourceID,
				Relation:          &relation,
			},
			expected: &relationspb.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
				ResourceId:        &resourceID,
				Relation:          &relation,
			},
		},
		{
			name: "filter with partial fields",
			input: &pb.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
			},
			expected: &relationspb.RelationTupleFilter{
				ResourceNamespace: &namespace,
				ResourceType:      &resourceType,
			},
		},
		{
			name:     "nil filter",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tupleFilterToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestFencingCheckToV1beta1 tests conversion of fencing checks.
func TestFencingCheckToV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    *pb.RelationFencingCheck
		expected *relationspb.FencingCheck
	}{
		{
			name: "valid fencing check",
			input: &pb.RelationFencingCheck{
				LockId:    "lock-1",
				LockToken: "token-abc",
			},
			expected: &relationspb.FencingCheck{
				LockId:    "lock-1",
				LockToken: "token-abc",
			},
		},
		{
			name:     "nil fencing check",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fencingCheckToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConsistencyToV1beta1 tests conversion of consistency requirements.
func TestConsistencyToV1beta1(t *testing.T) {
	token := "token-xyz"
	tests := []struct {
		name     string
		input    *pb.Consistency
		expected *relationspb.Consistency
	}{
		{
			name: "minimize latency",
			input: &pb.Consistency{
				Requirement: &pb.Consistency_MinimizeLatency{MinimizeLatency: true},
			},
			expected: &relationspb.Consistency{
				Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
			},
		},
		{
			name: "at least as fresh",
			input: &pb.Consistency{
				Requirement: &pb.Consistency_AtLeastAsFresh{
					AtLeastAsFresh: &pb.ConsistencyToken{Token: token},
				},
			},
			expected: &relationspb.Consistency{
				Requirement: &relationspb.Consistency_AtLeastAsFresh{
					AtLeastAsFresh: &relationspb.ConsistencyToken{Token: token},
				},
			},
		},
		{
			name: "at least as acknowledged (maps to minimize latency)",
			input: &pb.Consistency{
				Requirement: &pb.Consistency_AtLeastAsAcknowledged{AtLeastAsAcknowledged: true},
			},
			expected: &relationspb.Consistency{
				Requirement: &relationspb.Consistency_MinimizeLatency{MinimizeLatency: true},
			},
		},
		{
			name:     "nil consistency",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := consistencyToV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestConsistencyTokenFromV1beta1 tests conversion of consistency tokens.
func TestConsistencyTokenFromV1beta1(t *testing.T) {
	tests := []struct {
		name     string
		input    *relationspb.ConsistencyToken
		expected *pb.ConsistencyToken
	}{
		{
			name: "valid token",
			input: &relationspb.ConsistencyToken{
				Token: "token-123",
			},
			expected: &pb.ConsistencyToken{
				Token: "token-123",
			},
		},
		{
			name:     "nil token",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := consistencyTokenFromV1beta1(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
