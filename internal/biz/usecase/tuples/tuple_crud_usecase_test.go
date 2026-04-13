package tuples

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/authz/allow"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
)

// Test helpers

func testAuthzContext() context.Context {
	claims := &authnapi.Claims{
		SubjectId: authnapi.SubjectId("test-user"),
		AuthType:  authnapi.AuthTypeXRhIdentity,
	}
	return authnapi.NewAuthzContext(context.Background(), authnapi.AuthzContext{
		Protocol: authnapi.ProtocolGRPC,
		Subject:  claims,
	})
}

func createTestTuple() model.RelationsTuple {
	resourceId, _ := model.NewLocalResourceId("resource-1")
	subjectId, _ := model.NewLocalResourceId("subject-1")
	return model.NewRelationsTuple(
		model.NewRelationsResource(resourceId, model.NewRelationsObjectType("workspace", "rbac")),
		"member",
		model.NewRelationsSubject(
			model.NewRelationsResource(subjectId, model.NewRelationsObjectType("principal", "rbac")),
			"",
		),
	)
}

// Fakes

type recordingMetaAuthorizer struct {
	allowed   bool
	err       error
	relations []metaauthorizer.Relation
	calls     int
}

func (r *recordingMetaAuthorizer) Check(_ context.Context, _ metaauthorizer.MetaObject, relation metaauthorizer.Relation, _ authnapi.AuthzContext) (bool, error) {
	r.calls++
	r.relations = append(r.relations, relation)
	return r.allowed, r.err
}

// CreateTuples tests

func TestCreateTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
		Upsert: true,
	}

	result, err := uc.CreateTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestCreateTuples_UsesCreateTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, _ = uc.CreateTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationCreateTuples, meta.relations[0])
}

func TestCreateTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestCreateTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := CreateTuplesCommand{
		Tuples: []model.RelationsTuple{createTestTuple()},
	}

	_, err := uc.CreateTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// DeleteTuples tests

func TestDeleteTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	result, err := uc.DeleteTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestDeleteTuples_UsesDeleteTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, _ = uc.DeleteTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationDeleteTuples, meta.relations[0])
}

func TestDeleteTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, err := uc.DeleteTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestDeleteTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	namespace := "rbac"
	cmd := DeleteTuplesCommand{
		Filter: TupleFilter{
			ResourceNamespace: &namespace,
		},
	}

	_, err := uc.DeleteTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// ReadTuples tests

func TestReadTuples_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	stream, err := uc.ReadTuples(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, stream)
	assert.Equal(t, 1, meta.calls)
}

func TestReadTuples_UsesReadTuplesRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, _ = uc.ReadTuples(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationReadTuples, meta.relations[0])
}

func TestReadTuples_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err := uc.ReadTuples(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestReadTuples_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := ReadTuplesCommand{
		Filter:      TupleFilter{},
		Consistency: model.NewConsistencyMinimizeLatency(),
	}

	_, err := uc.ReadTuples(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}

// AcquireLock tests

func TestAcquireLock_Success(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	result, err := uc.AcquireLock(ctx, cmd)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, meta.calls)
}

func TestAcquireLock_UsesAcquireLockRelation(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, _ = uc.AcquireLock(ctx, cmd)

	require.Len(t, meta.relations, 1)
	assert.Equal(t, metaauthorizer.RelationAcquireLock, meta.relations[0])
}

func TestAcquireLock_MetaAuthzDenied(t *testing.T) {
	ctx := testAuthzContext()
	meta := &recordingMetaAuthorizer{allowed: false}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, err := uc.AcquireLock(ctx, cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthorizationDenied)
}

func TestAcquireLock_MetaAuthzContextMissing(t *testing.T) {
	meta := &recordingMetaAuthorizer{allowed: true}
	uc := New(&allow.AllowAllAuthz{}, meta, log.DefaultLogger)

	cmd := AcquireLockCommand{
		LockId: "lock-123",
	}

	_, err := uc.AcquireLock(context.Background(), cmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, metaauthorizer.ErrMetaAuthzContextMissing)
}
