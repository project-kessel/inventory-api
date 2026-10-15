// Package usecase implements the relationship delta use cases. These use
// cases provide targeted add/remove mutations of relationship-typed
// representation fields, complementing — not replacing — the full-snapshot
// semantics of the resources use cases (ReportResource / DeleteResource),
// which remain unchanged.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	kratosErrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/cmd/common"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/metaauthorizer"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// AddResourceRelationshipsOperationName identifies the serializable
// transaction used by the AddResourceRelationships use case.
const AddResourceRelationshipsOperationName = "AddResourceRelationships"

const listenTimeout = 10 * time.Second

// Domain errors re-exported from the model package.
var ErrResourceNotFound = model.ErrResourceNotFound

// AddResourceRelationshipsCommand contains the data needed to add one or more
// relationships to a resource. This is the domain command used by the usecase
// layer, decoupled from protobuf types.
type AddResourceRelationshipsCommand struct {
	// Resource identification
	LocalResourceId    model.LocalResourceId
	ResourceType       model.ResourceType
	ReporterType       model.ReporterType
	ReporterInstanceId model.ReporterInstanceId

	// Idempotency key. When nil (or empty) a new transaction ID is generated.
	TransactionId *model.TransactionId

	// Partial representations containing only relationship-typed fields.
	// The values are merged additively into the stored representations.
	// At least one must be provided.
	ReporterRepresentation *model.Representation
	CommonRepresentation   *model.Representation

	// Write behavior
	WriteVisibility resources.WriteVisibility
}

// AddResourceRelationshipsUseCase adds relationship-typed fields to a
// resource's representations without asserting the complete representation
// state. Unlike ReportResource, which replaces the entire representation,
// this use case merges the incoming relationship values into the stored ones
// and leaves all other fields untouched. If the resource does not yet exist
// for the calling reporter, it is created with the provided relationship
// fields as its initial representation.
//
// The use case shares infrastructure with the resources use cases (resource
// repository, schema repository, transaction manager, listen manager) but has
// no shared code path with ReportResource.
type AddResourceRelationshipsUseCase struct {
	schemaService       *model.SchemaService
	resourceRepository  model.ResourceRepository
	waitForNotifBreaker *gobreaker.CircuitBreaker[any]
	MetaAuthorizer      metaauthorizer.MetaAuthorizer
	Log                 *log.Helper
	ListenManager       pubsub.ListenManagerImpl
	Config              *resources.UsecaseConfig
	MetricsCollector    *metricscollector.MetricsCollector
}

// NewAddResourceRelationshipsUseCase creates a use case for adding
// relationship-typed fields to resource representations.
func NewAddResourceRelationshipsUseCase(
	resourceRepository model.ResourceRepository,
	schemaRepository model.SchemaRepository,
	listenManager pubsub.ListenManagerImpl,
	waitForNotifBreaker *gobreaker.CircuitBreaker[any],
	usecaseConfig *resources.UsecaseConfig,
	metricsCollector *metricscollector.MetricsCollector,
	metaAuthorizer metaauthorizer.MetaAuthorizer,
	logger log.Logger,
) *AddResourceRelationshipsUseCase {
	if metaAuthorizer == nil {
		metaAuthorizer = metaauthorizer.NewSimpleMetaAuthorizer()
	}
	if usecaseConfig == nil {
		usecaseConfig = resources.NewUsecaseConfig()
	}

	return &AddResourceRelationshipsUseCase{
		resourceRepository:  resourceRepository,
		schemaService:       model.NewSchemaService(schemaRepository, log.NewHelper(logger)),
		waitForNotifBreaker: waitForNotifBreaker,
		MetaAuthorizer:      metaAuthorizer,
		Log:                 log.NewHelper(logger),
		ListenManager:       listenManager,
		Config:              usecaseConfig,
		MetricsCollector:    metricsCollector,
	}
}

// AddResourceRelationships adds the relationship values named in the command
// to the resource identified by its key. If the resource does not exist it is
// created with the provided relationship fields as its initial
// representation; if the reporter's representation is tombstoned the request
// fails with ErrResourceNotFound. The full resulting representation is
// validated against the configured schemas before it is persisted.
func (uc *AddResourceRelationshipsUseCase) AddResourceRelationships(ctx context.Context, cmd AddResourceRelationshipsCommand) error {
	// Get authz context - required for authorization checks
	authzCtx, ok := authnapi.FromAuthzContext(ctx)
	if !ok || authzCtx.Subject == nil {
		return status.Error(codes.Unauthenticated, "authentication required")
	}

	reporterResourceKey, err := model.NewReporterResourceKey(
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
	)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to create reporter resource key: %v", err)
	}

	if err := uc.enforceMetaAuthzObject(ctx, metaauthorizer.RelationReportResource, metaauthorizer.NewInventoryResourceFromKey(reporterResourceKey)); err != nil {
		return err
	}

	if representationIsEmpty(cmd.ReporterRepresentation) && representationIsEmpty(cmd.CommonRepresentation) {
		return status.Error(codes.InvalidArgument, "at least one of reporter or common relationship fields must be provided")
	}

	var subscription pubsub.Subscription

	txid, err := getNextTransactionID()
	if err != nil {
		return err
	}

	if cmd.TransactionId == nil || *cmd.TransactionId == "" {
		cmd.TransactionId = &txid
	}

	readAfterWriteEnabled := uc.computeReadAfterWrite(cmd.WriteVisibility, authzCtx.Subject.SubjectId)
	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		subscription = uc.ListenManager.Subscribe(txid.String())
		defer subscription.Unsubscribe()
	}

	// Check for duplicate transaction ID outside the serializable transaction to avoid
	// predicate locks on the transaction_id indexes. The unique constraint on transaction_id
	// provides the actual correctness guarantee, if a duplicate sneaks past this
	// advisory check due to a concurrent commit.
	if uc.Config.IdempotencyCheckEnabled && cmd.TransactionId != nil {
		alreadyProcessed, err := uc.resourceRepository.HasTransactionIdBeenProcessed(uc.resourceRepository.GetDB(), *cmd.TransactionId)
		if err != nil {
			return fmt.Errorf("failed to check transaction ID: %w", err)
		}
		if alreadyProcessed {
			uc.Log.Infof("Transaction already processed, skipping update: transaction_id=%s", cmd.TransactionId.String())
			return nil
		}
	}

	var operationType model.EventOperationType
	addRelationships := func() error {
		return uc.resourceRepository.GetTransactionManager().HandleSerializableTransaction(
			AddResourceRelationshipsOperationName,
			uc.resourceRepository.GetDB(),
			func(tx *gorm.DB) error {
				res, err := uc.resourceRepository.FindResourceByKeys(tx, reporterResourceKey)
				if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("failed to lookup existing resource: %w", err)
				}

				if err == nil && res != nil {
					operationType = model.OperationTypeUpdated
					return uc.addRelationshipsToExistingResource(ctx, tx, cmd, res, reporterResourceKey, txid)
				}

				operationType = model.OperationTypeCreated
				return uc.createResourceWithRelationships(ctx, tx, cmd, txid)
			},
		)
	}

	err = addRelationships()
	if err != nil && !uc.Config.IdempotencyCheckEnabled && isDuplicateTransactionError(err) {
		uc.Log.Debugf("Idempotency check disabled and duplicate transaction ID detected, retrying with new transaction ID: %s", cmd.TransactionId.String())
		retryTxid, retryErr := getNextTransactionID()
		if retryErr != nil {
			return retryErr
		}
		cmd.TransactionId = &retryTxid
		err = addRelationships()
	}

	if err != nil {
		// Extract principal for failure logging
		principal := authzCtx.ExtractPrincipal()

		// CRUD operation failed - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation, EOI-11 warnings_or_errors)
		uc.Log.Warnw("msg", "Resource operation failed",
			"action", "ADD_RESOURCE_RELATIONSHIPS",
			"resource_type", cmd.ResourceType.String(),
			"resource_id", cmd.LocalResourceId,
			"reporter_type", cmd.ReporterType.String(),
			"reporter_instance_id", cmd.ReporterInstanceId.String(),
			"principal", principal,
			"outcome", "failure",
			"reason", err.Error(),
		)

		return err
	}

	// Extract principal for success logging
	principal := authzCtx.ExtractPrincipal()

	// Determine action based on operation type
	var action string
	switch operationType {
	case model.OperationTypeCreated:
		action = "CREATE"
	case model.OperationTypeUpdated:
		action = "UPDATE"
	default:
		action = "ADD_RESOURCE_RELATIONSHIPS"
	}

	// CRUD operation - SEC-MON-REQ-1 compliance (EOI-1 pii_manipulation)
	uc.Log.Infow("msg", "Resource operation completed",
		"action", action,
		"resource_type", cmd.ResourceType.String(),
		"resource_id", cmd.LocalResourceId,
		"reporter_type", cmd.ReporterType.String(),
		"reporter_instance_id", cmd.ReporterInstanceId.String(),
		"principal", principal,
		"outcome", "success",
	)

	// Increment outbox metrics only after successful transaction commit
	if operationType != nil {
		metricscollector.Incr(uc.MetricsCollector.OutboxEventWrites, string(operationType.OperationType()))
	}

	if readAfterWriteEnabled && uc.Config.ConsumerEnabled {
		timeoutCtx, cancel := context.WithTimeout(ctx, listenTimeout)
		defer cancel()

		_, err := uc.waitForNotifBreaker.Execute(func() (any, error) {
			err = subscription.BlockForNotification(timeoutCtx)
			if err != nil {
				// Return error for circuit breaker
				return nil, err
			}
			return nil, nil
		})

		if err != nil {
			switch {
			case errors.Is(err, pubsub.ErrWaitContextCancelled):
				uc.Log.WithContext(ctx).Debugf("Reached timeout waiting for notification from consumer")
				return nil
			case errors.Is(err, gobreaker.ErrOpenState):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is open, skipped waiting for notification from consumer")
				return nil
			case errors.Is(err, gobreaker.ErrTooManyRequests):
				uc.Log.WithContext(ctx).Debugf("Circuit breaker is half-open, skipped waiting for notification from consumer")
				return nil
			default:
				return err
			}
		}
	}

	return nil
}

// createResourceWithRelationships creates a new resource whose initial
// representation consists of the provided relationship fields. Schema
// validation still applies: if the resource type has required fields that are
// not relationships and are not provided, the create is rejected.
func (uc *AddResourceRelationshipsUseCase) createResourceWithRelationships(ctx context.Context, tx *gorm.DB, cmd AddResourceRelationshipsCommand, txid model.TransactionId) error {
	// The full resulting (initial) representation must satisfy the schema.
	// Absent streams are validated as empty so that required
	// non-relationship fields cause the auto-create to be rejected.
	reporterForValidation := cmd.ReporterRepresentation
	if reporterForValidation == nil {
		empty := model.NewEmptyRepresentation()
		reporterForValidation = &empty
	}
	commonForValidation := cmd.CommonRepresentation
	if commonForValidation == nil {
		empty := model.NewEmptyRepresentation()
		commonForValidation = &empty
	}
	if err := uc.schemaService.ValidateReportAgainstSchema(ctx, cmd.ResourceType, cmd.ReporterType, commonForValidation, reporterForValidation); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for add resource relationships: %v", err)
	}

	resourceId, err := uc.resourceRepository.NextResourceId()
	if err != nil {
		return err
	}

	reporterResourceId, err := uc.resourceRepository.NextReporterResourceId()
	if err != nil {
		return err
	}

	// The AddResourceRelationships contract carries no metadata (api/console
	// hrefs, reporter version); the resource is created without them.
	resource, err := model.NewResource(
		resourceId,
		cmd.LocalResourceId,
		cmd.ResourceType,
		cmd.ReporterType,
		cmd.ReporterInstanceId,
		*cmd.TransactionId,
		reporterResourceId,
		model.ApiHref(""),
		nil,
		cmd.ReporterRepresentation,
		cmd.CommonRepresentation,
		nil,
	)
	if err != nil {
		return err
	}

	return uc.resourceRepository.Save(tx, resource, model.OperationTypeCreated, txid)
}

// addRelationshipsToExistingResource merges the incoming relationship values
// into the stored representations of an existing (non-tombstoned) resource
// and persists the new representation version.
func (uc *AddResourceRelationshipsUseCase) addRelationshipsToExistingResource(ctx context.Context, tx *gorm.DB, cmd AddResourceRelationshipsCommand, res *model.Resource, key model.ReporterResourceKey, txid model.TransactionId) error {
	// Tombstoned representations cannot be re-activated through the delta
	// endpoints; a new ReportResource call is required.
	reporterResource, err := findReporterResource(res.ReporterResources(), key)
	if err != nil {
		return err
	}
	if reporterResource.Tombstoned() {
		return ErrResourceNotFound
	}

	// Load the current stored representations from both streams so the
	// merge starts from the persisted state.
	current, err := uc.resourceRepository.FindLatestRepresentations(tx, key)
	if err != nil {
		return fmt.Errorf("failed to load current representations: %w", err)
	}

	var storedReporter, storedCommon model.Representation
	if current != nil {
		storedReporter = current.ReporterData()
		storedCommon = current.CommonData()
	}

	mergedReporter, err := mergeRelationshipFields(storedReporter, cmd.ReporterRepresentation)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to merge reporter relationships: %v", err)
	}
	mergedCommon, err := mergeRelationshipFields(storedCommon, cmd.CommonRepresentation)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to merge common relationships: %v", err)
	}

	// The full resulting representation must satisfy the schema.
	var mergedReporterPtr, mergedCommonPtr *model.Representation
	if len(mergedReporter) > 0 {
		mergedReporterPtr = &mergedReporter
	}
	if len(mergedCommon) > 0 {
		mergedCommonPtr = &mergedCommon
	}
	if mergedReporterPtr == nil && mergedCommonPtr == nil {
		return status.Error(codes.InvalidArgument, "at least one of reporter or common relationship fields must be provided")
	}
	if err := uc.schemaService.ValidateReportAgainstSchema(ctx, cmd.ResourceType, cmd.ReporterType, mergedCommonPtr, mergedReporterPtr); err != nil {
		return status.Errorf(codes.InvalidArgument, "failed validation for add resource relationships: %v", err)
	}

	// The AddResourceRelationships contract carries no metadata, so the
	// stored hrefs are preserved on update. The stored reporter version is
	// not carried over: it is per-report metadata and, like ReportResource,
	// each write records the version supplied with that write (none here).
	if err := res.Update(
		key,
		reporterResource.ApiHref(),
		reporterResource.ConsoleHref(),
		nil,
		mergedReporterPtr,
		mergedCommonPtr,
		*cmd.TransactionId,
	); err != nil {
		return fmt.Errorf("failed to update resource: %w", err)
	}

	return uc.resourceRepository.Save(tx, *res, model.OperationTypeUpdated, txid)
}

// mergeRelationshipFields merges the incoming relationship values into the
// stored representation additively. Multi-valued (array) fields are unioned
// with the stored values; single-valued (scalar) fields are set only when
// absent — adding a different value to an already-set single-valued field is
// rejected, since a single-valued field cannot hold two values. Fields not
// named in the incoming representation are copied through unchanged.
func mergeRelationshipFields(stored model.Representation, incoming *model.Representation) (model.Representation, error) {
	merged := model.Representation{}
	for field, value := range stored {
		merged[field] = value
	}
	if incoming == nil {
		return merged, nil
	}

	for field, value := range *incoming {
		if value == nil {
			return nil, fmt.Errorf("relationship field %q has a null value", field)
		}

		switch incomingValues := value.(type) {
		case []interface{}:
			storedValue, exists := merged[field]
			if exists {
				storedValues, ok := storedValue.([]interface{})
				if !ok {
					return nil, fmt.Errorf("relationship field %q is single-valued and cannot accept an array", field)
				}
				merged[field] = unionValues(storedValues, incomingValues)
				continue
			}
			merged[field] = unionValues(nil, incomingValues)
		default:
			storedValue, exists := merged[field]
			if exists {
				if _, ok := storedValue.([]interface{}); ok {
					return nil, fmt.Errorf("relationship field %q is multi-valued and cannot accept a single value", field)
				}
				if !valuesEqual(storedValue, value) {
					return nil, fmt.Errorf("relationship field %q already has value %v and cannot hold an additional value", field, storedValue)
				}
				// Same value: idempotent, keep the stored one.
				continue
			}
			merged[field] = value
		}
	}

	return merged, nil
}

// unionValues returns the set union of the stored and incoming values,
// preserving the stored order and appending only values not already present.
func unionValues(stored, incoming []interface{}) []interface{} {
	union := make([]interface{}, 0, len(stored)+len(incoming))
	seen := make(map[string]struct{}, len(stored)+len(incoming))
	for _, value := range stored {
		key := valueKey(value)
		if _, duplicate := seen[key]; !duplicate {
			seen[key] = struct{}{}
			union = append(union, value)
		}
	}
	for _, value := range incoming {
		key := valueKey(value)
		if _, duplicate := seen[key]; !duplicate {
			seen[key] = struct{}{}
			union = append(union, value)
		}
	}
	return union
}

// valueKey produces a comparison key for a relationship value. Relationship
// values are strings in practice; other JSON types fall back to their
// formatted representation.
func valueKey(value interface{}) string {
	if s, ok := value.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", value)
}

// valuesEqual compares two single-valued relationship values for equality.
func valuesEqual(a, b interface{}) bool {
	return valueKey(a) == valueKey(b)
}

// representationIsEmpty reports whether a partial representation is absent or
// carries no fields.
func representationIsEmpty(rep *model.Representation) bool {
	return rep == nil || len(*rep) == 0
}

// findReporterResource locates the reporter resource matching the key, using
// the same case-insensitive matching semantics as the model's update path.
func findReporterResource(reporterResources []model.ReporterResource, key model.ReporterResourceKey) (model.ReporterResource, error) {
	for _, reporterResource := range reporterResources {
		storedKey := reporterResource.Key()

		if strings.EqualFold(storedKey.LocalResourceId().Serialize(), key.LocalResourceId().Serialize()) &&
			strings.EqualFold(storedKey.ResourceType().Serialize(), key.ResourceType().Serialize()) &&
			strings.EqualFold(storedKey.ReporterType().Serialize(), key.ReporterType().Serialize()) {

			searchReporterInstanceId := key.ReporterInstanceId().Serialize()
			storedReporterInstanceId := storedKey.ReporterInstanceId().Serialize()

			if searchReporterInstanceId == "" || strings.EqualFold(storedReporterInstanceId, searchReporterInstanceId) {
				return reporterResource, nil
			}
		}
	}

	return model.ReporterResource{}, fmt.Errorf("reporter resource with key (localResourceId=%s, resourceType=%s, reporterType=%s, reporterInstanceId=%s) not found in resource",
		key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId())
}

// enforceMetaAuthzObject calls the MetaAuthorizer to validate access using a MetaObject.
func (uc *AddResourceRelationshipsUseCase) enforceMetaAuthzObject(ctx context.Context, relation metaauthorizer.Relation, metaObject metaauthorizer.MetaObject) error {
	return metaauthorizer.EnforceMetaAuthzObject(ctx, uc.MetaAuthorizer, relation, metaObject)
}

// computeReadAfterWrite determines whether the caller requested — and the
// deployment supports — waiting for the write to become visible to the
// consumer before returning.
func (uc *AddResourceRelationshipsUseCase) computeReadAfterWrite(writeVisibility resources.WriteVisibility, callerSubject authnapi.SubjectId) bool {
	if writeVisibility == resources.WriteVisibilityUnspecified || writeVisibility == resources.WriteVisibilityMinimizeLatency {
		return false
	}
	return !common.IsNil(uc.ListenManager) && uc.Config.ReadAfterWriteEnabled && isSPInAllowlist(callerSubject, uc.Config.ReadAfterWriteAllowlist)
}

// isSPInAllowlist checks if the caller subject is in the allowlist.
func isSPInAllowlist(callerSubject authnapi.SubjectId, allowlist []string) bool {
	for _, sp := range allowlist {
		if sp == string(callerSubject) || sp == "*" {
			return true
		}
	}

	return false
}

func isDuplicateTransactionError(err error) bool {
	var kratosErr *kratosErrors.Error
	if errors.As(err, &kratosErr) {
		return kratosErr.Reason == model.ReasonNonUniqueTransactionID
	}
	return false
}

func getNextTransactionID() (model.TransactionId, error) {
	txid, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return model.NewTransactionId(txid.String()), nil
}
