package data

import (
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz"
	"github.com/project-kessel/inventory-api/internal/biz/model"
)

// FakeStore implements model.Store for testing.
type FakeStore struct {
	resourceRepo *fakeModelResourceRepository
	eventSource  model.EventSource
}

// NewFakeStore creates a new FakeStore for testing.
func NewFakeStore() *FakeStore {
	return &FakeStore{
		resourceRepo: newFakeModelResourceRepository(),
	}
}

// NewFakeStoreWithEventSource creates a new FakeStore with a custom event source.
func NewFakeStoreWithEventSource(eventSource model.EventSource) *FakeStore {
	return &FakeStore{
		resourceRepo: newFakeModelResourceRepository(),
		eventSource:  eventSource,
	}
}

// Ensure FakeStore implements model.Store
var _ model.Store = (*FakeStore)(nil)

// Begin starts a new fake transaction.
func (s *FakeStore) Begin() (model.Tx, error) {
	return &fakeStoreTx{
		store: s,
		repo:  s.resourceRepo,
	}, nil
}

// EventSource returns the event source for consuming outbox events.
// Returns nil if no event source was configured.
func (s *FakeStore) EventSource() model.EventSource {
	return s.eventSource
}

// GetResourceRepository returns the underlying fake repository for test assertions.
func (s *FakeStore) GetResourceRepository() *fakeModelResourceRepository {
	return s.resourceRepo
}

// fakeStoreTx implements model.Tx for testing.
type fakeStoreTx struct {
	store *FakeStore
	repo  *fakeModelResourceRepository
	done  bool // true after Commit or Rollback
}

var _ model.Tx = (*fakeStoreTx)(nil)

func (tx *fakeStoreTx) ResourceRepository() model.ResourceRepository {
	return tx.repo
}

func (tx *fakeStoreTx) Commit() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return nil
}

// Rollback aborts the transaction. Safe to call after Commit (no-op).
func (tx *fakeStoreTx) Rollback() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return nil
}

// fakeModelResourceRepository implements model.ResourceRepository for testing.
type fakeModelResourceRepository struct {
	mu                sync.RWMutex
	resources         map[string]*model.Resource
	processedTxIds    map[string]bool
	representations   map[string]*model.Representations
}

func newFakeModelResourceRepository() *fakeModelResourceRepository {
	return &fakeModelResourceRepository{
		resources:       make(map[string]*model.Resource),
		processedTxIds:  make(map[string]bool),
		representations: make(map[string]*model.Representations),
	}
}

var _ model.ResourceRepository = (*fakeModelResourceRepository)(nil)

func (r *fakeModelResourceRepository) NextResourceId() (model.ResourceId, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.ResourceId{}, err
	}
	return model.NewResourceId(id)
}

func (r *fakeModelResourceRepository) NextReporterResourceId() (model.ReporterResourceId, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return model.ReporterResourceId{}, err
	}
	return model.NewReporterResourceId(id)
}

func (r *fakeModelResourceRepository) Save(resource model.Resource, operationType biz.EventOperationType, txid string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Extract and store the command's transaction ID for idempotency checks
	// The transaction ID is embedded in the CommonRepresentation
	_, _, _, commonRepSnapshot, err := resource.Serialize()
	if err == nil && commonRepSnapshot.TransactionId != "" {
		r.processedTxIds[commonRepSnapshot.TransactionId] = true
	}

	// Store resource by key
	for _, rr := range resource.ReporterResources() {
		key := r.makeKey(rr.Key())
		// Store a copy to avoid mutation issues
		resourceCopy := resource
		r.resources[key] = &resourceCopy
	}

	return nil
}

func (r *fakeModelResourceRepository) FindResourceByKeys(key model.ReporterResourceKey) (*model.Resource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	k := r.makeKey(key)
	res, exists := r.resources[k]
	if !exists {
		// Return nil, nil to indicate not found - caller should check for nil resource
		return nil, nil
	}
	return res, nil
}

func (r *fakeModelResourceRepository) FindCurrentAndPreviousVersionedRepresentations(key model.ReporterResourceKey, currentVersion *uint, operationType biz.EventOperationType) (*model.Representations, *model.Representations, error) {
	// Simplified for testing - return nil representations
	return nil, nil, nil
}

func (r *fakeModelResourceRepository) FindLatestRepresentations(key model.ReporterResourceKey) (*model.Representations, error) {
	return nil, nil
}

func (r *fakeModelResourceRepository) ContainsEventForTransactionId(transactionId string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.processedTxIds[transactionId], nil
}

func (r *fakeModelResourceRepository) makeKey(key model.ReporterResourceKey) string {
	return strings.ToLower(fmt.Sprintf("%s/%s/%s/%s",
		key.LocalResourceId().Serialize(),
		key.ResourceType().Serialize(),
		key.ReporterType().Serialize(),
		key.ReporterInstanceId().Serialize()))
}

// FindResourceByKeysForTest is a helper for tests to find resources.
func (r *fakeModelResourceRepository) FindResourceByKeysForTest(key model.ReporterResourceKey) (*model.Resource, error) {
	return r.FindResourceByKeys(key)
}
