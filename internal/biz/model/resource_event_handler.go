package model

// import (
// 	"context"

// 	"github.com/project-kessel/inventory-api/internal/pubsub"
// )

// type TupleReplicationService struct {
// 	eventChan           <-chan ResourceEvent
// 	relationsRepository RelationsRepository
// 	schemaService       SchemaService
// 	resourceRepository  ResourceRepository
// 	notifier            pubsub.Notifier
// }

// func NewTupleReplicationService(eventChan <-chan ResourceEvent) *TupleReplicationService {
// 	return &TupleReplicationService{
// 		eventChan: eventChan,
// 	}
// }

// func (s *TupleReplicationService) Run() {
// 	for event := range s.eventChan {
// 		s.handleEvent(event)
// 	}
// }

// func (s *TupleReplicationService) handleEvent(event ResourceEvent) {
// 	ctx := context.Background()

// 	tx, err := s.resourceRepository.Begin()
// 	if err != nil {
// 		// handle error (could log or propagate)
// 		return
// 	}
// 	defer tx.Rollback()

// 	// Fetch resource representation for the prior event version
// 	// TODO: also return resource to remove later call
// 	current, prior, err := tx.FindCurrentAndPreviousVersionedRepresentations(event.ReporterResourceKey(), event.PriorVersion(), biz.EventOperationTypeCreated)
// 	if err != nil {
// 		// handle error or missing resource, depending on semantics
// 		return
// 	}

// 	// Calculate the diff of tuples between prior and current representations
// 	update := s.schemaService.CalculateTuples(prior, current, event.ReporterResourceKey())
// 	if update.IsEmpty() {
// 		return
// 	}

// 	token := s.relationsRepository.UpdateTuples(update)

// 	resource, err := tx.FindResourceByKeys(event.ReporterResourceKey())
// 	if err != nil {
// 		return
// 	}
// 	resource.AcknowledgeRelationsUpTo(token)
// 	tx.Save(resource, biz.EventOperationTypeCreated, event.TransactionID())
// 	tx.Commit()

// 	notifierErr := s.notifier.Notify(ctx, event.TransactionID())
// 	if notifierErr != nil {
// 		// handle error (could log or propagate)
// 		return
// 	}
// }
