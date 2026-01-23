package authz

import (
	"context"
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthorizerReplicator wraps an Authorizer to implement model.RelationsReplicator.
type AuthorizerReplicator struct {
	authorizer api.Authorizer
	lockID     string
	lockToken  string
}

// NewAuthorizerReplicator creates a new AuthorizerReplicator.
func NewAuthorizerReplicator(authorizer api.Authorizer, lockID, lockToken string) *AuthorizerReplicator {
	return &AuthorizerReplicator{
		authorizer: authorizer,
		lockID:     lockID,
		lockToken:  lockToken,
	}
}

// SetLock updates the lock credentials for fencing.
func (a *AuthorizerReplicator) SetLock(lockID, lockToken string) {
	a.lockID = lockID
	a.lockToken = lockToken
}

// ReplicateTuples implements model.RelationsReplicator.
func (a *AuthorizerReplicator) ReplicateTuples(ctx context.Context, creates, deletes []model.RelationsTuple) (model.ConsistencyToken, error) {
	var token string

	// Create tuples if any
	if len(creates) > 0 {
		createToken, err := a.createTuples(ctx, creates)
		if err != nil {
			return model.ConsistencyToken(""), fmt.Errorf("failed to create tuples: %w", err)
		}
		token = createToken
	}

	// Delete tuples if any
	if len(deletes) > 0 {
		deleteToken, err := a.deleteTuples(ctx, deletes)
		if err != nil {
			return model.ConsistencyToken(""), fmt.Errorf("failed to delete tuples: %w", err)
		}
		if token == "" {
			token = deleteToken
		}
	}

	return model.ConsistencyToken(token), nil
}

func (a *AuthorizerReplicator) createTuples(ctx context.Context, tuples []model.RelationsTuple) (string, error) {
	relationships := a.convertToRelationships(tuples)

	resp, err := a.authorizer.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Upsert: true,
		Tuples: relationships,
		FencingCheck: &kessel.FencingCheck{
			LockId:    a.lockID,
			LockToken: a.lockToken,
		},
	})
	if err != nil {
		if status.Convert(err).Code() == codes.FailedPrecondition {
			return "", fmt.Errorf("invalid fencing token: %w", err)
		}

		// If the tuple exists already, fetch token via Check
		if status.Convert(err).Code() == codes.AlreadyExists && len(relationships) > 0 {
			return a.fetchExistingToken(ctx, relationships[0])
		}
		return "", fmt.Errorf("error creating tuple: %w", err)
	}
	return resp.GetConsistencyToken().GetToken(), nil
}

func (a *AuthorizerReplicator) deleteTuples(ctx context.Context, tuples []model.RelationsTuple) (string, error) {
	var token string

	// Delete each tuple individually using filters
	for _, tuple := range tuples {
		filter := a.convertToFilter(tuple)

		resp, err := a.authorizer.DeleteTuples(ctx, &kessel.DeleteTuplesRequest{
			Filter: filter,
			FencingCheck: &kessel.FencingCheck{
				LockId:    a.lockID,
				LockToken: a.lockToken,
			},
		})
		if err != nil {
			if status.Convert(err).Code() == codes.FailedPrecondition {
				return "", fmt.Errorf("invalid fencing token: %w", err)
			}
			return "", fmt.Errorf("error deleting tuple: %w", err)
		}

		if token == "" {
			token = resp.GetConsistencyToken().GetToken()
		}
	}

	return token, nil
}

func (a *AuthorizerReplicator) convertToFilter(tuple model.RelationsTuple) *kessel.RelationTupleFilter {
	resourceNamespace := strings.ToLower(tuple.Resource().Type().Namespace())
	resourceType := strings.ToLower(tuple.Resource().Type().Name())
	resourceID := tuple.Resource().Id().Serialize()
	relation := strings.ToLower(tuple.Relation())
	subjectNamespace := strings.ToLower(tuple.Subject().Subject().Type().Namespace())
	subjectType := strings.ToLower(tuple.Subject().Subject().Type().Name())
	subjectID := tuple.Subject().Subject().Id().Serialize()

	return &kessel.RelationTupleFilter{
		ResourceNamespace: &resourceNamespace,
		ResourceType:      &resourceType,
		ResourceId:        &resourceID,
		Relation:          &relation,
		SubjectFilter: &kessel.SubjectFilter{
			SubjectNamespace: &subjectNamespace,
			SubjectType:      &subjectType,
			SubjectId:        &subjectID,
		},
	}
}

func (a *AuthorizerReplicator) fetchExistingToken(ctx context.Context, rel *kessel.Relationship) (string, error) {
	namespace := rel.GetResource().GetType().GetNamespace()
	relation := rel.GetRelation()
	subject := rel.GetSubject()
	resourceType := rel.GetResource().GetType().GetName()
	resourceID := rel.GetResource().GetId()

	_, token, err := a.authorizer.Check(ctx, namespace, relation, "", resourceType, resourceID, subject)
	if err != nil {
		return "", fmt.Errorf("failed to fetch consistency token: %w", err)
	}
	return token.GetToken(), nil
}

func (a *AuthorizerReplicator) convertToRelationships(tuples []model.RelationsTuple) []*kessel.Relationship {
	relationships := make([]*kessel.Relationship, 0, len(tuples))

	for _, tuple := range tuples {
		rel := &kessel.Relationship{
			Resource: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Namespace: strings.ToLower(tuple.Resource().Type().Namespace()),
					Name:      strings.ToLower(tuple.Resource().Type().Name()),
				},
				Id: tuple.Resource().Id().Serialize(),
			},
			Relation: strings.ToLower(tuple.Relation()),
			Subject: &kessel.SubjectReference{
				Subject: &kessel.ObjectReference{
					Type: &kessel.ObjectType{
						Namespace: strings.ToLower(tuple.Subject().Subject().Type().Namespace()),
						Name:      strings.ToLower(tuple.Subject().Subject().Type().Name()),
					},
					Id: tuple.Subject().Subject().Id().Serialize(),
				},
			},
		}
		relationships = append(relationships, rel)
	}

	return relationships
}
