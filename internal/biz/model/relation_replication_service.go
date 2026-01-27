package model

import (
	"context"
	"fmt"

	"github.com/project-kessel/inventory-api/internal/biz"
)

// RelationsReplicator abstracts operations on the relations service (e.g., SpiceDB).
// This interface is satisfied by the Authorizer implementations.
type RelationsReplicator interface {
	// ReplicateTuples applies the given tuple changes and returns a consistency token.
	// The lock parameter provides the fencing credentials for the operation.
	// If the fencing check fails, ErrFencingFailed should be returned.
	ReplicateTuples(ctx context.Context, creates, deletes []RelationsTuple, lock Lock) (ConsistencyToken, error)
}

// RelationReplicationService handles the domain logic for replicating inventory changes to relations.
type RelationReplicationService struct {
	relationsReplicator RelationsReplicator
	schemaService       *SchemaService
}

// NewRelationReplicationService creates a new RelationReplicationService.
func NewRelationReplicationService(
	relationsReplicator RelationsReplicator,
	schemaService *SchemaService,
) *RelationReplicationService {
	return &RelationReplicationService{
		relationsReplicator: relationsReplicator,
		schemaService:       schemaService,
	}
}

// Replicate processes a delivery, replicating the changes to the relations service.
// The lock from the delivery is used to fence operations against the relations service.
func (s *RelationReplicationService) Replicate(ctx context.Context, tx Tx, delivery Delivery) error {
	event := delivery.Event()
	lock := delivery.Lock()

	repo := tx.ResourceRepository()
	key := event.TupleEvent().ReporterResourceKey()
	operation := event.Operation()

	// Fetch current and previous representations based on operation type
	current, previous, err := s.fetchRepresentations(repo, key, event, operation)
	if err != nil {
		return fmt.Errorf("failed to fetch representations: %w", err)
	}

	// Calculate tuples to replicate
	tuplesToReplicate, err := s.schemaService.CalculateTuplesForResource(ctx, current, previous, key)
	if err != nil {
		return fmt.Errorf("failed to calculate tuples: %w", err)
	}

	if tuplesToReplicate.IsEmpty() {
		return nil
	}

	// Execute the relations operation and get consistency token
	token, err := s.executeRelationsOperation(ctx, operation, tuplesToReplicate, lock)
	if err != nil {
		return fmt.Errorf("failed to execute relations operation: %w", err)
	}

	// Update the resource's consistency token (skip for deletes)
	if operation != OperationDeleted && !token.IsZero() {
		if err := s.updateConsistencyToken(repo, key, token, event.TxID().String()); err != nil {
			return fmt.Errorf("failed to update consistency token: %w", err)
		}
	}

	return nil
}

func (s *RelationReplicationService) fetchRepresentations(
	repo ResourceRepository,
	key ReporterResourceKey,
	event OutboxEvent,
	operation OperationType,
) (*Representations, *Representations, error) {
	version := event.TupleEvent().CommonVersion()
	var versionPtr *uint
	if version != nil {
		v := version.Uint()
		versionPtr = &v
	}

	switch operation {
	case OperationCreated:
		return repo.FindCurrentAndPreviousVersionedRepresentations(key, versionPtr, biz.OperationTypeCreated)

	case OperationUpdated:
		return repo.FindCurrentAndPreviousVersionedRepresentations(key, versionPtr, biz.OperationTypeUpdated)

	case OperationDeleted:
		previous, err := repo.FindLatestRepresentations(key)
		return nil, previous, err

	default:
		return nil, nil, fmt.Errorf("unknown operation type: %s", operation)
	}
}

func (s *RelationReplicationService) executeRelationsOperation(
	ctx context.Context,
	operation OperationType,
	tuples TuplesToReplicate,
	lock Lock,
) (ConsistencyToken, error) {
	var creates, deletes []RelationsTuple

	if tuples.TuplesToCreate() != nil {
		creates = *tuples.TuplesToCreate()
	}
	if tuples.TuplesToDelete() != nil {
		deletes = *tuples.TuplesToDelete()
	}

	switch operation {
	case OperationCreated:
		return s.relationsReplicator.ReplicateTuples(ctx, creates, nil, lock)

	case OperationUpdated:
		return s.relationsReplicator.ReplicateTuples(ctx, creates, deletes, lock)

	case OperationDeleted:
		return s.relationsReplicator.ReplicateTuples(ctx, nil, deletes, lock)

	default:
		return ConsistencyToken(""), fmt.Errorf("unknown operation type: %s", operation)
	}
}

func (s *RelationReplicationService) updateConsistencyToken(
	repo ResourceRepository,
	key ReporterResourceKey,
	token ConsistencyToken,
	txid string,
) error {
	resource, err := repo.FindResourceByKeys(key)
	if err != nil {
		return fmt.Errorf("failed to find resource: %w", err)
	}
	if resource == nil {
		// Resource was deleted, skip token update
		return nil
	}

	resource.SetConsistencyToken(token)

	// Save the resource with the updated consistency token
	return repo.Save(*resource, biz.OperationTypeUpdated, txid)
}
