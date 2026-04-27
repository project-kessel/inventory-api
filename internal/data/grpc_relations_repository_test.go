package data

import (
	"context"
	"io"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselapi "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	rpcstatus "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func grpcTestRef(namespace, resourceType, resourceID string) model.ResourceReference {
	reporter := model.NewReporterReference(model.DeserializeReporterType(namespace), nil)
	return model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(resourceID),
		&reporter,
	)
}

func grpcTestRefNoReporter(resourceType, resourceID string) model.ResourceReference {
	return model.NewResourceReference(
		model.DeserializeResourceType(resourceType),
		model.DeserializeLocalResourceId(resourceID),
		nil,
	)
}

func grpcTestSubject(namespace, resourceType, resourceID string) model.SubjectReference {
	return model.NewSubjectReferenceWithoutRelation(grpcTestRef(namespace, resourceType, resourceID))
}

func grpcTestRelationship(objNs, objType, objID, relation, subNs, subType, subID string) model.Relationship {
	return model.NewRelationship(
		grpcTestRef(objNs, objType, objID),
		model.DeserializeRelation(relation),
		grpcTestSubject(subNs, subType, subID),
	)
}

// --- resourceReferenceToV1Beta1 ---

func TestResourceReferenceToV1Beta1_WithReporter(t *testing.T) {
	ref := grpcTestRef("hbi", "host", "host-1")
	result := resourceReferenceToV1Beta1(ref)

	assert.Equal(t, "hbi", result.Type.Namespace)
	assert.Equal(t, "host", result.Type.Name)
	assert.Equal(t, "host-1", result.Id)
}

func TestResourceReferenceToV1Beta1_WithoutReporter(t *testing.T) {
	ref := grpcTestRefNoReporter("host", "host-1")
	result := resourceReferenceToV1Beta1(ref)

	assert.Equal(t, "", result.Type.Namespace)
	assert.Equal(t, "host", result.Type.Name)
	assert.Equal(t, "host-1", result.Id)
}

// --- subjectToV1Beta1 ---

func TestSubjectToV1Beta1_WithoutRelation(t *testing.T) {
	sub := grpcTestSubject("rbac", "principal", "user-1")
	result := subjectToV1Beta1(sub)

	assert.Equal(t, "rbac", result.Subject.Type.Namespace)
	assert.Equal(t, "principal", result.Subject.Type.Name)
	assert.Equal(t, "user-1", result.Subject.Id)
	assert.Nil(t, result.Relation)
}

func TestSubjectToV1Beta1_WithRelation(t *testing.T) {
	rel := model.DeserializeRelation("members")
	sub := model.NewSubjectReference(grpcTestRef("rbac", "group", "group-1"), &rel)
	result := subjectToV1Beta1(sub)

	assert.Equal(t, "rbac", result.Subject.Type.Namespace)
	assert.Equal(t, "group", result.Subject.Type.Name)
	assert.Equal(t, "group-1", result.Subject.Id)
	require.NotNil(t, result.Relation)
	assert.Equal(t, "members", *result.Relation)
}

// --- consistencyToV1Beta1 ---

func TestConsistencyToV1Beta1_MinimizeLatency(t *testing.T) {
	result := consistencyToV1Beta1(model.NewConsistencyMinimizeLatency())
	require.NotNil(t, result)
	assert.True(t, result.GetMinimizeLatency())
}

func TestConsistencyToV1Beta1_AtLeastAsFresh(t *testing.T) {
	token := model.DeserializeConsistencyToken("token-123")
	result := consistencyToV1Beta1(model.NewConsistencyAtLeastAsFresh(token))
	require.NotNil(t, result)
	require.NotNil(t, result.GetAtLeastAsFresh())
	assert.Equal(t, "token-123", result.GetAtLeastAsFresh().Token)
}

func TestConsistencyToV1Beta1_Unspecified(t *testing.T) {
	result := consistencyToV1Beta1(model.NewConsistencyUnspecified())
	require.NotNil(t, result)
	assert.True(t, result.GetMinimizeLatency())
}

func TestConsistencyToV1Beta1_Nil(t *testing.T) {
	result := consistencyToV1Beta1(nil)
	require.NotNil(t, result)
	assert.True(t, result.GetMinimizeLatency())
}

// --- tokenFromV1Beta1 ---

func TestTokenFromV1Beta1_Nil(t *testing.T) {
	assert.Equal(t, model.MinimizeLatencyToken, tokenFromV1Beta1(nil))
}

func TestTokenFromV1Beta1_WithToken(t *testing.T) {
	assert.Equal(t, model.DeserializeConsistencyToken("abc"), tokenFromV1Beta1(&kesselapi.ConsistencyToken{Token: "abc"}))
}

func TestTokenFromV1Beta1_EmptyToken(t *testing.T) {
	assert.Equal(t, model.DeserializeConsistencyToken(""), tokenFromV1Beta1(&kesselapi.ConsistencyToken{Token: ""}))
}

// --- paginationToV1Beta1 ---

func TestPaginationToV1Beta1_Nil(t *testing.T) {
	assert.Nil(t, paginationToV1Beta1(nil))
}

func TestPaginationToV1Beta1_WithContinuation(t *testing.T) {
	token := "continuation-abc"
	result := paginationToV1Beta1(&model.Pagination{Limit: 100, Continuation: &token})
	require.NotNil(t, result)
	assert.Equal(t, uint32(100), result.Limit)
	require.NotNil(t, result.ContinuationToken)
	assert.Equal(t, "continuation-abc", *result.ContinuationToken)
}

func TestPaginationToV1Beta1_WithoutContinuation(t *testing.T) {
	result := paginationToV1Beta1(&model.Pagination{Limit: 250})
	require.NotNil(t, result)
	assert.Equal(t, uint32(250), result.Limit)
	assert.Nil(t, result.ContinuationToken)
}

// --- fencingCheckToV1Beta1 ---

func TestFencingCheckToV1Beta1(t *testing.T) {
	fc := model.NewFencingCheck(model.DeserializeLockId("lock-1"), model.DeserializeLockToken("token-1"))
	result := fencingCheckToV1Beta1(&fc)
	assert.Equal(t, "lock-1", result.LockId)
	assert.Equal(t, "token-1", result.LockToken)
}

// --- tuplesToV1Beta1 ---

func TestTuplesToV1Beta1(t *testing.T) {
	objRep := model.NewReporterReference(model.DeserializeReporterType("rbac"), nil)
	object := model.NewResourceReference(model.DeserializeResourceType("workspace"), model.DeserializeLocalResourceId("ws-1"), &objRep)
	subRep := model.NewReporterReference(model.DeserializeReporterType("rbac"), nil)
	subRes := model.NewResourceReference(model.DeserializeResourceType("principal"), model.DeserializeLocalResourceId("user-1"), &subRep)
	tuple := model.NewRelationsTuple(object, model.DeserializeRelation("member"), model.NewSubjectReferenceWithoutRelation(subRes))

	result := tuplesToV1Beta1([]model.RelationsTuple{tuple})
	require.Len(t, result, 1)
	assert.Equal(t, "rbac", result[0].Resource.Type.Namespace)
	assert.Equal(t, "workspace", result[0].Resource.Type.Name)
	assert.Equal(t, "ws-1", result[0].Resource.Id)
	assert.Equal(t, "member", result[0].Relation)
	assert.Equal(t, "user-1", result[0].Subject.Subject.Id)
}

func TestTuplesToV1Beta1_Empty(t *testing.T) {
	assert.Empty(t, tuplesToV1Beta1([]model.RelationsTuple{}))
}

// --- tupleFilterToV1Beta1 ---

func TestTupleFilterToV1Beta1_Empty(t *testing.T) {
	result := tupleFilterToV1Beta1(model.NewTupleFilter())
	assert.Nil(t, result.ResourceNamespace)
	assert.Nil(t, result.ResourceType)
	assert.Nil(t, result.ResourceId)
	assert.Nil(t, result.Relation)
	assert.Nil(t, result.SubjectFilter)
}

func TestTupleFilterToV1Beta1_AllFields(t *testing.T) {
	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("rbac")).
		WithObjectType(model.DeserializeResourceType("workspace")).
		WithObjectId(model.DeserializeLocalResourceId("ws-1")).
		WithRelation(model.DeserializeRelation("member")).
		WithSubject(model.NewTupleSubjectFilter().
			WithReporterType(model.DeserializeReporterType("rbac")).
			WithSubjectType(model.DeserializeResourceType("principal")).
			WithSubjectId(model.DeserializeLocalResourceId("user-1")).
			WithRelation(model.DeserializeRelation("members")))

	result := tupleFilterToV1Beta1(filter)

	require.NotNil(t, result.ResourceNamespace)
	assert.Equal(t, "rbac", *result.ResourceNamespace)
	require.NotNil(t, result.ResourceType)
	assert.Equal(t, "workspace", *result.ResourceType)
	require.NotNil(t, result.ResourceId)
	assert.Equal(t, "ws-1", *result.ResourceId)
	require.NotNil(t, result.Relation)
	assert.Equal(t, "member", *result.Relation)

	require.NotNil(t, result.SubjectFilter)
	require.NotNil(t, result.SubjectFilter.SubjectNamespace)
	assert.Equal(t, "rbac", *result.SubjectFilter.SubjectNamespace)
	require.NotNil(t, result.SubjectFilter.SubjectType)
	assert.Equal(t, "principal", *result.SubjectFilter.SubjectType)
	require.NotNil(t, result.SubjectFilter.SubjectId)
	assert.Equal(t, "user-1", *result.SubjectFilter.SubjectId)
	require.NotNil(t, result.SubjectFilter.Relation)
	assert.Equal(t, "members", *result.SubjectFilter.Relation)
}

func TestTupleFilterToV1Beta1_PartialFields(t *testing.T) {
	filter := model.NewTupleFilter().
		WithReporterType(model.DeserializeReporterType("hbi")).
		WithRelation(model.DeserializeRelation("view"))

	result := tupleFilterToV1Beta1(filter)

	require.NotNil(t, result.ResourceNamespace)
	assert.Equal(t, "hbi", *result.ResourceNamespace)
	assert.Nil(t, result.ResourceType)
	assert.Nil(t, result.ResourceId)
	require.NotNil(t, result.Relation)
	assert.Equal(t, "view", *result.Relation)
	assert.Nil(t, result.SubjectFilter)
}

// --- relationshipsToCheckBulkV1Beta1 ---

func TestRelationshipsToCheckBulkV1Beta1(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "user-1"),
		grpcTestRelationship("hbi", "host", "h-2", "edit", "rbac", "principal", "user-2"),
	}
	result := relationshipsToCheckBulkV1Beta1(rels)

	require.Len(t, result, 2)
	assert.Equal(t, "h-1", result[0].Resource.Id)
	assert.Equal(t, "view", result[0].Relation)
	assert.Equal(t, "user-1", result[0].Subject.Subject.Id)
	assert.Equal(t, "h-2", result[1].Resource.Id)
	assert.Equal(t, "edit", result[1].Relation)
	assert.Equal(t, "user-2", result[1].Subject.Subject.Id)
}

// --- checkBulkResultFromV1Beta1 ---

func TestCheckBulkResultFromV1Beta1_AllowedAndDenied(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "u-1"),
		grpcTestRelationship("hbi", "host", "h-2", "view", "rbac", "principal", "u-2"),
	}
	pairs := []*kesselapi.CheckBulkResponsePair{
		{Response: &kesselapi.CheckBulkResponsePair_Item{
			Item: &kesselapi.CheckBulkResponseItem{Allowed: kesselapi.CheckBulkResponseItem_ALLOWED_TRUE},
		}},
		{Response: &kesselapi.CheckBulkResponsePair_Item{
			Item: &kesselapi.CheckBulkResponseItem{Allowed: kesselapi.CheckBulkResponseItem_ALLOWED_FALSE},
		}},
	}

	result, err := checkBulkResultFromV1Beta1(pairs, rels, &kesselapi.ConsistencyToken{Token: "bulk-token"})
	require.NoError(t, err)
	require.Len(t, result.Pairs(), 2)
	assert.True(t, result.Pairs()[0].Result().Allowed())
	assert.Nil(t, result.Pairs()[0].Result().Err())
	assert.False(t, result.Pairs()[1].Result().Allowed())
	assert.Nil(t, result.Pairs()[1].Result().Err())
	assert.Equal(t, model.DeserializeConsistencyToken("bulk-token"), result.ConsistencyToken())
}

func TestCheckBulkResultFromV1Beta1_WithError(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "u-1"),
	}
	pairs := []*kesselapi.CheckBulkResponsePair{
		{Response: &kesselapi.CheckBulkResponsePair_Error{
			Error: &rpcstatus.Status{
				Code:    int32(codes.PermissionDenied),
				Message: "denied",
			},
		}},
	}

	result, err := checkBulkResultFromV1Beta1(pairs, rels, nil)
	require.NoError(t, err)
	require.Len(t, result.Pairs(), 1)
	assert.False(t, result.Pairs()[0].Result().Allowed())
	require.NotNil(t, result.Pairs()[0].Result().Err())
	assert.Contains(t, result.Pairs()[0].Result().Err().Error(), "denied")
	assert.Equal(t, int32(codes.PermissionDenied), result.Pairs()[0].Result().ErrorCode())
}

func TestCheckBulkResultFromV1Beta1_NilErrorAndItem(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "u-1"),
	}
	pairs := []*kesselapi.CheckBulkResponsePair{{}}

	result, err := checkBulkResultFromV1Beta1(pairs, rels, nil)
	require.NoError(t, err)
	require.Len(t, result.Pairs(), 1)
	assert.False(t, result.Pairs()[0].Result().Allowed())
	require.NotNil(t, result.Pairs()[0].Result().Err())
	assert.Contains(t, result.Pairs()[0].Result().Err().Error(), "malformed")
	assert.Equal(t, int32(codes.Internal), result.Pairs()[0].Result().ErrorCode())
}

func TestCheckBulkResultFromV1Beta1_MismatchedLength(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "u-1"),
	}
	pairs := []*kesselapi.CheckBulkResponsePair{
		{Response: &kesselapi.CheckBulkResponsePair_Item{
			Item: &kesselapi.CheckBulkResponseItem{Allowed: kesselapi.CheckBulkResponseItem_ALLOWED_TRUE},
		}},
		{Response: &kesselapi.CheckBulkResponsePair_Item{
			Item: &kesselapi.CheckBulkResponseItem{Allowed: kesselapi.CheckBulkResponseItem_ALLOWED_TRUE},
		}},
	}

	_, err := checkBulkResultFromV1Beta1(pairs, rels, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatched")
}

func TestCheckBulkResultFromV1Beta1_NilToken(t *testing.T) {
	rels := []model.Relationship{
		grpcTestRelationship("hbi", "host", "h-1", "view", "rbac", "principal", "u-1"),
	}
	pairs := []*kesselapi.CheckBulkResponsePair{
		{Response: &kesselapi.CheckBulkResponsePair_Item{
			Item: &kesselapi.CheckBulkResponseItem{Allowed: kesselapi.CheckBulkResponseItem_ALLOWED_TRUE},
		}},
	}

	result, err := checkBulkResultFromV1Beta1(pairs, rels, nil)
	require.NoError(t, err)
	assert.Equal(t, model.MinimizeLatencyToken, result.ConsistencyToken())
}

// --- streaming adapter mocks ---

type grpcTestLookupResourcesStream struct {
	responses []*kesselapi.LookupResourcesResponse
	current   int
}

func (m *grpcTestLookupResourcesStream) Recv() (*kesselapi.LookupResourcesResponse, error) {
	if m.current >= len(m.responses) {
		return nil, io.EOF
	}
	resp := m.responses[m.current]
	m.current++
	return resp, nil
}
func (m *grpcTestLookupResourcesStream) Header() (metadata.MD, error) { return nil, nil }
func (m *grpcTestLookupResourcesStream) Trailer() metadata.MD         { return nil }
func (m *grpcTestLookupResourcesStream) CloseSend() error             { return nil }
func (m *grpcTestLookupResourcesStream) Context() context.Context     { return context.Background() }
func (m *grpcTestLookupResourcesStream) SendMsg(interface{}) error    { return nil }
func (m *grpcTestLookupResourcesStream) RecvMsg(interface{}) error    { return nil }

func TestLookupObjectsStream_Recv(t *testing.T) {
	mock := &grpcTestLookupResourcesStream{
		responses: []*kesselapi.LookupResourcesResponse{
			{
				Resource: &kesselapi.ObjectReference{
					Type: &kesselapi.ObjectType{Namespace: "hbi", Name: "host"},
					Id:   "host-1",
				},
				Pagination: &kesselapi.ResponsePagination{ContinuationToken: "page-1"},
			},
		},
	}

	stream := &lookupObjectsStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "host-1", item.Object().ResourceId().String())
	assert.Equal(t, "host", item.Object().ResourceType().String())
	require.NotNil(t, item.Object().Reporter())
	assert.Equal(t, "hbi", item.Object().Reporter().ReporterType().String())
	assert.Equal(t, "page-1", item.ContinuationToken())

	_, err = stream.Recv()
	assert.ErrorIs(t, err, io.EOF)
}

type grpcTestLookupSubjectsStream struct {
	responses []*kesselapi.LookupSubjectsResponse
	current   int
}

func (m *grpcTestLookupSubjectsStream) Recv() (*kesselapi.LookupSubjectsResponse, error) {
	if m.current >= len(m.responses) {
		return nil, io.EOF
	}
	resp := m.responses[m.current]
	m.current++
	return resp, nil
}
func (m *grpcTestLookupSubjectsStream) Header() (metadata.MD, error) { return nil, nil }
func (m *grpcTestLookupSubjectsStream) Trailer() metadata.MD         { return nil }
func (m *grpcTestLookupSubjectsStream) CloseSend() error             { return nil }
func (m *grpcTestLookupSubjectsStream) Context() context.Context     { return context.Background() }
func (m *grpcTestLookupSubjectsStream) SendMsg(interface{}) error    { return nil }
func (m *grpcTestLookupSubjectsStream) RecvMsg(interface{}) error    { return nil }

func TestLookupSubjectsStream_WithoutRelation(t *testing.T) {
	mock := &grpcTestLookupSubjectsStream{
		responses: []*kesselapi.LookupSubjectsResponse{
			{
				Subject: &kesselapi.SubjectReference{
					Subject: &kesselapi.ObjectReference{
						Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "principal"},
						Id:   "user-1",
					},
				},
				Pagination: &kesselapi.ResponsePagination{ContinuationToken: "tok"},
			},
		},
	}

	stream := &lookupSubjectsStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "user-1", item.Subject().Resource().ResourceId().String())
	assert.Equal(t, "principal", item.Subject().Resource().ResourceType().String())
	assert.False(t, item.Subject().HasRelation())
	assert.Equal(t, "tok", item.ContinuationToken())
}

func TestLookupSubjectsStream_WithRelation(t *testing.T) {
	rel := "members"
	mock := &grpcTestLookupSubjectsStream{
		responses: []*kesselapi.LookupSubjectsResponse{
			{
				Subject: &kesselapi.SubjectReference{
					Subject: &kesselapi.ObjectReference{
						Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "group"},
						Id:   "group-1",
					},
					Relation: &rel,
				},
				Pagination: &kesselapi.ResponsePagination{},
			},
		},
	}

	stream := &lookupSubjectsStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.True(t, item.Subject().HasRelation())
	assert.Equal(t, "members", item.Subject().Relation().String())
}

type grpcTestReadTuplesStream struct {
	responses []*kesselapi.ReadTuplesResponse
	current   int
}

func (m *grpcTestReadTuplesStream) Recv() (*kesselapi.ReadTuplesResponse, error) {
	if m.current >= len(m.responses) {
		return nil, io.EOF
	}
	resp := m.responses[m.current]
	m.current++
	return resp, nil
}
func (m *grpcTestReadTuplesStream) Header() (metadata.MD, error) { return nil, nil }
func (m *grpcTestReadTuplesStream) Trailer() metadata.MD         { return nil }
func (m *grpcTestReadTuplesStream) CloseSend() error             { return nil }
func (m *grpcTestReadTuplesStream) Context() context.Context     { return context.Background() }
func (m *grpcTestReadTuplesStream) SendMsg(interface{}) error    { return nil }
func (m *grpcTestReadTuplesStream) RecvMsg(interface{}) error    { return nil }

func TestReadTuplesStream_BasicTuple(t *testing.T) {
	mock := &grpcTestReadTuplesStream{
		responses: []*kesselapi.ReadTuplesResponse{
			{
				Tuple: &kesselapi.Relationship{
					Resource: &kesselapi.ObjectReference{
						Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "workspace"},
						Id:   "ws-1",
					},
					Relation: "member",
					Subject: &kesselapi.SubjectReference{
						Subject: &kesselapi.ObjectReference{
							Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "principal"},
							Id:   "user-1",
						},
					},
				},
				Pagination:       &kesselapi.ResponsePagination{ContinuationToken: "page-tok"},
				ConsistencyToken: &kesselapi.ConsistencyToken{Token: "ct-1"},
			},
		},
	}

	stream := &readTuplesStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, "ws-1", item.Object().ResourceId().String())
	assert.Equal(t, "workspace", item.Object().ResourceType().String())
	assert.Equal(t, "rbac", item.Object().Reporter().ReporterType().String())
	assert.Equal(t, model.DeserializeRelation("member"), item.Relation())
	assert.Equal(t, "user-1", item.Subject().Resource().ResourceId().String())
	assert.False(t, item.Subject().HasRelation())
	assert.Equal(t, "page-tok", item.ContinuationToken())
	assert.Equal(t, model.DeserializeConsistencyToken("ct-1"), item.ConsistencyToken())
}

func TestReadTuplesStream_WithSubjectRelation(t *testing.T) {
	rel := "members"
	mock := &grpcTestReadTuplesStream{
		responses: []*kesselapi.ReadTuplesResponse{
			{
				Tuple: &kesselapi.Relationship{
					Resource: &kesselapi.ObjectReference{
						Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "workspace"},
						Id:   "ws-1",
					},
					Relation: "member",
					Subject: &kesselapi.SubjectReference{
						Subject: &kesselapi.ObjectReference{
							Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "group"},
							Id:   "group-1",
						},
						Relation: &rel,
					},
				},
				Pagination: &kesselapi.ResponsePagination{},
			},
		},
	}

	stream := &readTuplesStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.True(t, item.Subject().HasRelation())
	assert.Equal(t, "members", item.Subject().Relation().String())
}

func TestReadTuplesStream_EmptyConsistencyToken(t *testing.T) {
	mock := &grpcTestReadTuplesStream{
		responses: []*kesselapi.ReadTuplesResponse{
			{
				Tuple: &kesselapi.Relationship{
					Resource: &kesselapi.ObjectReference{
						Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "workspace"},
						Id:   "ws-1",
					},
					Relation: "member",
					Subject: &kesselapi.SubjectReference{
						Subject: &kesselapi.ObjectReference{
							Type: &kesselapi.ObjectType{Namespace: "rbac", Name: "principal"},
							Id:   "user-1",
						},
					},
				},
				Pagination: &kesselapi.ResponsePagination{},
			},
		},
	}

	stream := &readTuplesStream{stream: mock}
	item, err := stream.Recv()
	require.NoError(t, err)
	assert.Equal(t, model.ConsistencyToken(""), item.ConsistencyToken())
}

func TestReadTuplesStream_EOF(t *testing.T) {
	stream := &readTuplesStream{stream: &grpcTestReadTuplesStream{}}
	_, err := stream.Recv()
	assert.ErrorIs(t, err, io.EOF)
}

// --- empty streams ---

func TestEmptyLookupObjectsStream(t *testing.T) {
	_, err := (&emptyLookupObjectsStream{}).Recv()
	assert.ErrorIs(t, err, io.EOF)
}

func TestEmptyLookupSubjectsStream(t *testing.T) {
	_, err := (&emptyLookupSubjectsStream{}).Recv()
	assert.ErrorIs(t, err, io.EOF)
}

func TestEmptyReadTuplesStream(t *testing.T) {
	_, err := (&emptyReadTuplesStream{}).Recv()
	assert.ErrorIs(t, err, io.EOF)
}
