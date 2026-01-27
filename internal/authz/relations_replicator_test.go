package authz

import (
	"context"
	"errors"
	"testing"

	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeAuthorizer is a test double for the api.Authorizer interface
type fakeAuthorizer struct {
	createTuplesFunc func(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error)
	deleteTuplesFunc func(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error)
	checkFunc        func(ctx context.Context, namespace, relation, resourceNamespace, resourceType, resourceID string, subject *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error)
}

func (f *fakeAuthorizer) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	return nil, nil
}

func (f *fakeAuthorizer) Check(ctx context.Context, namespace, relation, resourceNamespace, resourceType, resourceID string, subject *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error) {
	if f.checkFunc != nil {
		return f.checkFunc(ctx, namespace, relation, resourceNamespace, resourceType, resourceID, subject)
	}
	return kessel.CheckResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: "check-token"}, nil
}

func (f *fakeAuthorizer) CheckForUpdate(ctx context.Context, namespace, relation, resourceNamespace, resourceID string, subject *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error) {
	return kessel.CheckForUpdateResponse_ALLOWED_TRUE, &kessel.ConsistencyToken{Token: "check-token"}, nil
}

func (f *fakeAuthorizer) CheckBulk(ctx context.Context, req *kessel.CheckBulkRequest) (*kessel.CheckBulkResponse, error) {
	return nil, nil
}

func (f *fakeAuthorizer) LookupResources(ctx context.Context, req *kessel.LookupResourcesRequest) (grpc.ServerStreamingClient[kessel.LookupResourcesResponse], error) {
	return nil, nil
}

func (f *fakeAuthorizer) CreateTuples(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	if f.createTuplesFunc != nil {
		return f.createTuplesFunc(ctx, req)
	}
	return &kessel.CreateTuplesResponse{
		ConsistencyToken: &kessel.ConsistencyToken{Token: "test-token"},
	}, nil
}

func (f *fakeAuthorizer) DeleteTuples(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	if f.deleteTuplesFunc != nil {
		return f.deleteTuplesFunc(ctx, req)
	}
	return &kessel.DeleteTuplesResponse{
		ConsistencyToken: &kessel.ConsistencyToken{Token: "test-token"},
	}, nil
}

func (f *fakeAuthorizer) AcquireLock(ctx context.Context, req *kessel.AcquireLockRequest) (*kessel.AcquireLockResponse, error) {
	return &kessel.AcquireLockResponse{LockToken: "test-lock-token"}, nil
}

func createTestTuple() model.RelationsTuple {
	localResourceId, _ := model.NewLocalResourceId("test-resource-123")
	resourceType := model.NewRelationsObjectType("host", "hbi")
	resource := model.NewRelationsResource(localResourceId, resourceType)

	subjectId, _ := model.NewLocalResourceId("workspace-1")
	subjectType := model.NewRelationsObjectType("workspace", "rbac")
	subjectResource := model.NewRelationsResource(subjectId, subjectType)
	subject := model.NewRelationsSubject(subjectResource)

	return model.NewRelationsTuple(resource, "workspace", subject)
}

func TestAuthorizerReplicator_ReplicateTuples_Success(t *testing.T) {
	t.Parallel()

	t.Run("should successfully create tuples", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("token123"))

		tuples := []model.RelationsTuple{createTestTuple()}

		token, err := replicator.ReplicateTuples(context.Background(), tuples, nil, lock)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if token.IsZero() {
			t.Error("Expected non-zero consistency token")
		}
	})

	t.Run("should successfully delete tuples", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("token123"))

		tuples := []model.RelationsTuple{createTestTuple()}

		token, err := replicator.ReplicateTuples(context.Background(), nil, tuples, lock)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if token.IsZero() {
			t.Error("Expected non-zero consistency token")
		}
	})
}

func TestAuthorizerReplicator_ReplicateTuples_FencingFailure(t *testing.T) {
	t.Parallel()

	t.Run("should return ErrFencingFailed on FailedPrecondition for create", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{
			createTuplesFunc: func(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
				return nil, status.Error(codes.FailedPrecondition, "invalid fencing token")
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("expired-token"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), tuples, nil, lock)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, model.ErrFencingFailed) {
			t.Errorf("Expected ErrFencingFailed, got %v", err)
		}
	})

	t.Run("should return ErrFencingFailed on FailedPrecondition for delete", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{
			deleteTuplesFunc: func(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
				return nil, status.Error(codes.FailedPrecondition, "invalid fencing token")
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("expired-token"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), nil, tuples, lock)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		if !errors.Is(err, model.ErrFencingFailed) {
			t.Errorf("Expected ErrFencingFailed, got %v", err)
		}
	})
}

func TestAuthorizerReplicator_ReplicateTuples_PassesLockCredentials(t *testing.T) {
	t.Parallel()

	t.Run("should pass lock ID and token to CreateTuples request", func(t *testing.T) {
		t.Parallel()

		var capturedLockId, capturedLockToken string
		authorizer := &fakeAuthorizer{
			createTuplesFunc: func(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
				if req.FencingCheck != nil {
					capturedLockId = req.FencingCheck.LockId
					capturedLockToken = req.FencingCheck.LockToken
				}
				return &kessel.CreateTuplesResponse{
					ConsistencyToken: &kessel.ConsistencyToken{Token: "test-token"},
				}, nil
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("test-consumer/0"), model.NewLockToken("my-token-123"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), tuples, nil, lock)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if capturedLockId != "test-consumer/0" {
			t.Errorf("Expected lock ID %s, got %s", "test-consumer/0", capturedLockId)
		}
		if capturedLockToken != "my-token-123" {
			t.Errorf("Expected lock token %s, got %s", "my-token-123", capturedLockToken)
		}
	})

	t.Run("should pass lock ID and token to DeleteTuples request", func(t *testing.T) {
		t.Parallel()

		var capturedLockId, capturedLockToken string
		authorizer := &fakeAuthorizer{
			deleteTuplesFunc: func(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
				if req.FencingCheck != nil {
					capturedLockId = req.FencingCheck.LockId
					capturedLockToken = req.FencingCheck.LockToken
				}
				return &kessel.DeleteTuplesResponse{
					ConsistencyToken: &kessel.ConsistencyToken{Token: "test-token"},
				}, nil
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("delete-consumer/1"), model.NewLockToken("delete-token-456"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), nil, tuples, lock)

		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if capturedLockId != "delete-consumer/1" {
			t.Errorf("Expected lock ID %s, got %s", "delete-consumer/1", capturedLockId)
		}
		if capturedLockToken != "delete-token-456" {
			t.Errorf("Expected lock token %s, got %s", "delete-token-456", capturedLockToken)
		}
	})
}

func TestAuthorizerReplicator_ReplicateTuples_OtherErrors(t *testing.T) {
	t.Parallel()

	t.Run("should wrap other errors on create", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{
			createTuplesFunc: func(ctx context.Context, req *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
				return nil, status.Error(codes.Internal, "internal error")
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("token"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), tuples, nil, lock)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		// Should not be ErrFencingFailed for other errors
		if errors.Is(err, model.ErrFencingFailed) {
			t.Error("Expected non-fencing error, got ErrFencingFailed")
		}
	})

	t.Run("should wrap other errors on delete", func(t *testing.T) {
		t.Parallel()

		authorizer := &fakeAuthorizer{
			deleteTuplesFunc: func(ctx context.Context, req *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
				return nil, status.Error(codes.Unavailable, "service unavailable")
			},
		}
		replicator := NewAuthorizerReplicator(authorizer)
		lock := model.NewLock(model.NewLockId("consumer/0"), model.NewLockToken("token"))

		tuples := []model.RelationsTuple{createTestTuple()}

		_, err := replicator.ReplicateTuples(context.Background(), nil, tuples, lock)

		if err == nil {
			t.Error("Expected error, got nil")
		}
		// Should not be ErrFencingFailed for other errors
		if errors.Is(err, model.ErrFencingFailed) {
			t.Error("Expected non-fencing error, got ErrFencingFailed")
		}
	})
}
