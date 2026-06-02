package data

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	kratosErrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
)

type FindResourceByKeysResult struct {
	ReporterResourceID    uuid.UUID `gorm:"column:reporter_resource_id"`
	RepresentationVersion uint      `gorm:"column:representation_version"`
	Generation            uint      `gorm:"column:generation"`
	Tombstone             bool      `gorm:"column:tombstone"`
	CommonVersion         *uint     `gorm:"column:common_version"`
	LastCommonVersion     *uint     `gorm:"column:last_common_version"`
	ResourceID            uuid.UUID `gorm:"column:resource_id"`
	ResourceType          string    `gorm:"column:resource_type"`
	LocalResourceID       string    `gorm:"column:local_resource_id"`
	ReporterType          string    `gorm:"column:reporter_type"`
	ReporterInstanceID    string    `gorm:"column:reporter_instance_id"`
	APIHref               string    `gorm:"column:api_href"`
	ConsoleHref           *string   `gorm:"column:console_href"`
	ConsistencyToken      string    `gorm:"column:consistency_token"`
	CreatedAt             time.Time `gorm:"column:created_at"`
	UpdatedAt             time.Time `gorm:"column:updated_at"`
}

// GetCurrentAndPreviousWorkspaceID extracts current and previous workspace IDs from Representations
func GetCurrentAndPreviousWorkspaceID(current, previous *bizmodel.Representations) (currentWorkspaceID, previousWorkspaceID string) {
	return current.WorkspaceID(), previous.WorkspaceID()
}

func ToSnapshotsFromResults(results []FindResourceByKeysResult) (*bizmodel.ResourceSnapshot, []bizmodel.ReporterResourceSnapshot) {
	if len(results) == 0 {
		return nil, nil
	}

	var reporterSnapshots []bizmodel.ReporterResourceSnapshot
	var resourceSnapshot bizmodel.ResourceSnapshot

	for i, result := range results {
		resSnap, repSnap := result.ToSnapshots()

		if i == 0 {
			resourceSnapshot = resSnap
		}
		reporterSnapshots = append(reporterSnapshots, repSnap)
	}

	return &resourceSnapshot, reporterSnapshots
}

func (result FindResourceByKeysResult) ToSnapshots() (bizmodel.ResourceSnapshot, bizmodel.ReporterResourceSnapshot) {
	resourceSnapshot := bizmodel.ResourceSnapshot{
		ID:                result.ResourceID,
		Type:              result.ResourceType,
		CommonVersion:     result.CommonVersion,
		LastCommonVersion: result.LastCommonVersion,
		ConsistencyToken:  result.ConsistencyToken,
		CreatedAt:         result.CreatedAt,
		UpdatedAt:         result.UpdatedAt,
	}

	keySnapshot := bizmodel.ReporterResourceKeySnapshot{
		LocalResourceID:    result.LocalResourceID,
		ReporterType:       result.ReporterType,
		ResourceType:       result.ResourceType,
		ReporterInstanceID: result.ReporterInstanceID,
	}

	reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
		ID:                    result.ReporterResourceID,
		ReporterResourceKey:   keySnapshot,
		ResourceID:            result.ResourceID,
		APIHref:               result.APIHref,
		ConsoleHref:           result.ConsoleHref,
		RepresentationVersion: result.RepresentationVersion,
		Generation:            result.Generation,
		Tombstone:             result.Tombstone,
		CreatedAt:             result.CreatedAt,
		UpdatedAt:             result.UpdatedAt,
	}

	return resourceSnapshot, reporterResourceSnapshot
}

// --- ResourceRepository implementation ---

type resourceRepository struct {
	db                      *gorm.DB
	outboxPublisher         OutboxPublisher
	metricsCollector        *metricscollector.MetricsCollector
	maxSerializationRetries int
	operationName           string
}

// GormResourceRepositoryConfig holds configuration for creating a GORM-backed ResourceRepository.
type GormResourceRepositoryConfig struct {
	DB                      *gorm.DB
	OutboxPublisher         OutboxPublisher
	MetricsCollector        *metricscollector.MetricsCollector
	MaxSerializationRetries int
	OperationName           string
}

func NewResourceRepository(cfg GormResourceRepositoryConfig) bizmodel.ResourceRepository {
	publisher := cfg.OutboxPublisher
	if publisher == nil {
		publisher = publishOutboxEvent
	}
	maxRetries := cfg.MaxSerializationRetries
	if maxRetries == 0 {
		maxRetries = 3
	}
	return &resourceRepository{
		db:                      cfg.DB,
		outboxPublisher:         publisher,
		metricsCollector:        cfg.MetricsCollector,
		maxSerializationRetries: maxRetries,
		operationName:           cfg.OperationName,
	}
}

var _ bizmodel.ResourceRepository = (*resourceRepository)(nil)

func (r *resourceRepository) Begin() (bizmodel.ResourceTx, error) {
	gormTx := r.db.Begin(&sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if gormTx.Error != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", gormTx.Error)
	}
	return &gormResourceTx{
		gormTx:           gormTx,
		outboxPublisher:  r.outboxPublisher,
		metricsCollector: r.metricsCollector,
		operationName:    r.operationName,
	}, nil
}

func (r *resourceRepository) MaxSerializationRetries() int {
	return r.maxSerializationRetries
}

func (r *resourceRepository) RecordSerializationExhaustion() {
	metricscollector.Incr(r.metricsCollector.SerializationExhaustions, r.operationName)
}

func isSerializationFailure(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == "40001" {
			return true
		}
	}

	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		if sqliteErr.Code == sqlite3.ErrBusy || sqliteErr.Code == sqlite3.ErrLocked {
			return true
		}
	}
	return false
}

// --- ResourceTx implementation ---

// gormResourceTx implements model.ResourceTx within a GORM transaction.
type gormResourceTx struct {
	gormTx           *gorm.DB
	outboxPublisher  OutboxPublisher
	metricsCollector *metricscollector.MetricsCollector
	operationName    string
	done             bool
}

var _ bizmodel.ResourceTx = (*gormResourceTx)(nil)

func (tx *gormResourceTx) Commit() error {
	if tx.done {
		return nil
	}
	if err := tx.gormTx.Commit().Error; err != nil {
		if isSerializationFailure(err) {
			metricscollector.Incr(tx.metricsCollector.SerializationFailures, tx.operationName)
			return fmt.Errorf("commit failed: %w", bizmodel.ErrSerializationFailure)
		}
		return fmt.Errorf("commit failed: %w", err)
	}
	tx.done = true
	return nil
}

func (tx *gormResourceTx) Rollback() error {
	if tx.done {
		return nil
	}
	tx.done = true
	return tx.gormTx.Rollback().Error
}

// wrapSerializationError checks whether err is a serialization failure and,
// if so, wraps it with the domain sentinel and increments the metric.
func (tx *gormResourceTx) wrapSerializationError(err error) error {
	if err != nil && isSerializationFailure(err) {
		metricscollector.Incr(tx.metricsCollector.SerializationFailures, tx.operationName)
		return fmt.Errorf("%w: %v", bizmodel.ErrSerializationFailure, err)
	}
	return err
}

func (tx *gormResourceTx) NextResourceId() (bizmodel.ResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ResourceId{}, err
	}
	return bizmodel.NewResourceId(uuidV7)
}

func (tx *gormResourceTx) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ReporterResourceId{}, err
	}
	return bizmodel.NewReporterResourceId(uuidV7)
}

func (tx *gormResourceTx) Save(resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	dataResource := datamodel.DeserializeResourceFromSnapshot(resourceSnapshot)
	dataReporterResource := datamodel.DeserializeReporterResourceFromSnapshot(reporterResourceSnapshot)

	if err := tx.gormTx.Save(&dataResource).Error; err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return wrapped
		}
		return fmt.Errorf("failed to save resource: %w", err)
	}

	if err := tx.gormTx.Save(&dataReporterResource).Error; err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return wrapped
		}
		return fmt.Errorf("failed to save reporter resource: %w", err)
	}

	if reporterRepresentationSnapshot != nil {
		dataReporterRepresentation := datamodel.DeserializeReporterRepresentationFromSnapshot(*reporterRepresentationSnapshot)
		if err := tx.gormTx.Create(&dataReporterRepresentation).Error; err != nil {
			if kratosErrors.Is(err, gorm.ErrDuplicatedKey) {
				return kratosErrors.BadRequest(bizmodel.ReasonNonUniqueTransactionID, err.Error()).WithCause(err)
			}
			if wrapped := tx.wrapSerializationError(err); wrapped != err {
				return wrapped
			}
			return fmt.Errorf("failed to save reporter representation: %w", err)
		}
	}

	if commonRepresentationSnapshot != nil {
		dataCommonRepresentation := datamodel.DeserializeCommonRepresentationFromSnapshot(*commonRepresentationSnapshot)
		if err := tx.gormTx.Create(&dataCommonRepresentation).Error; err != nil {
			if kratosErrors.Is(err, gorm.ErrDuplicatedKey) {
				return kratosErrors.BadRequest(bizmodel.ReasonNonUniqueTransactionID, err.Error()).WithCause(err)
			}
			if wrapped := tx.wrapSerializationError(err); wrapped != err {
				return wrapped
			}
			return fmt.Errorf("failed to save common representation: %w", err)
		}
	}

	var resourceEvent bizmodel.ResourceEvent
	switch operationType {
	case bizmodel.OperationTypeDeleted:
		deleteEvents := resource.ResourceDeleteEvents()
		log.Infof("DeleteEvents to publish to outbox : %+v", deleteEvents)
		if len(deleteEvents) == 0 {
			return nil
		}
		resourceEvent = deleteEvents[0]
	default:
		resourceEvent = resource.ResourceReportEvents()[0]
	}
	return tx.handleOutboxEvents(resourceEvent, operationType, txid)
}

func (tx *gormResourceTx) handleOutboxEvents(resourceEvent bizmodel.ResourceEvent, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	resourceMessage, tupleMessage, err := model_legacy.NewOutboxEventsFromResourceEvent(resourceEvent, operationType, txid)
	if err != nil {
		return err
	}

	if err := tx.outboxPublisher(tx.gormTx, resourceMessage); err != nil {
		return err
	}

	return tx.outboxPublisher(tx.gormTx, tupleMessage)
}

func (tx *gormResourceTx) getDBSession() *gorm.DB {
	return tx.gormTx
}

func (tx *gormResourceTx) buildReporterResourceKeyQuery(db *gorm.DB, key bizmodel.ReporterResourceKey) *gorm.DB {
	query := db.
		Where("rr.local_resource_id = ?", key.LocalResourceId().Serialize()).
		Where("rr.resource_type = ?", key.ResourceType().Serialize()).
		Where("rr.reporter_type = ?", key.ReporterType().Serialize())

	if reporterInstanceId := key.ReporterInstanceId().Serialize(); reporterInstanceId != "" {
		query = query.Where("rr.reporter_instance_id = ?", reporterInstanceId)
	}

	return query
}

func (tx *gormResourceTx) FindResourceByKeys(key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	var results []FindResourceByKeysResult

	db := tx.getDBSession()

	query := db.Table("reporter_resources AS rr").
		Select(`
		rr2.id AS reporter_resource_id,
		rr2.representation_version,
		rr2.generation,
		rr2.tombstone,
		res.common_version,
		(SELECT MAX(cr.version) FROM common_representations cr WHERE cr.resource_id = res.id) AS last_common_version,
		res.id AS resource_id,
		res.ktn AS consistency_token,
		res.created_at,
		res.updated_at,
		rr2.resource_type,
		rr2.local_resource_id,
		rr2.reporter_type,
		rr2.reporter_instance_id,
		rr2.api_href,
		rr2.console_href
	`).
		Joins(`
		JOIN reporter_resources AS rr2 ON rr2.resource_id = rr.resource_id
		JOIN resource AS res ON res.id = rr2.resource_id
	`)

	// ORDER BY aligns with the fake repository's deterministic tie-breaking:
	// non-tombstoned rows first, then highest representation_version, then generation.
	err := tx.buildReporterResourceKeyQuery(query, key).
		Order("rr2.tombstone ASC, rr2.representation_version DESC, rr2.generation DESC").
		Find(&results).Error

	if err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return nil, wrapped
		}
		return nil, fmt.Errorf("failed to find resource by keys: %w", err)
	}

	if len(results) == 0 {
		return nil, bizmodel.ErrResourceNotFound
	}

	resourceSnapshot, reporterResourceSnapshots := ToSnapshotsFromResults(results)
	resource := bizmodel.DeserializeResource(resourceSnapshot, reporterResourceSnapshots, nil, nil)

	return resource, nil
}

func (tx *gormResourceTx) FindCurrentAndPreviousVersionedRepresentations(key bizmodel.ReporterResourceKey, currentCommonVersion *bizmodel.Version, operationType bizmodel.EventOperationType) (*bizmodel.Representations, *bizmodel.Representations, error) {
	if currentCommonVersion == nil {
		return nil, nil, nil
	}

	type commonRepresentationRow struct {
		Data                       internal.JsonObject
		Version                    uint
		ResourceId                 uuid.UUID
		ReportedByReporterType     string
		ReportedByReporterInstance string
		TransactionId              string
	}

	var results []commonRepresentationRow

	db := tx.getDBSession()

	query := db.Table("reporter_resources rr").
		Select("cr.data, cr.version, cr.resource_id, cr.reported_by_reporter_type, cr.reported_by_reporter_instance, cr.transaction_id").
		Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id")

	query = tx.buildReporterResourceKeyQuery(query, key)

	cv := currentCommonVersion.Uint()
	if operationType.OperationType() == bizmodel.OperationTypeCreated {
		query = query.Where("cr.version = ?", cv)
	} else {
		query = query.Where("(cr.version = ? OR cr.version = ?)", cv, cv-1)
	}

	err := query.Find(&results).Error
	if err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return nil, nil, wrapped
		}
		return nil, nil, fmt.Errorf("failed to find common representations by version: %w", err)
	}

	var current, previous *bizmodel.Representations
	for _, row := range results {
		v := bizmodel.NewVersion(row.Version)
		rep, err := bizmodel.NewRepresentations(bizmodel.Representation(row.Data), &v, nil, nil)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create representation: %w", err)
		}

		if row.Version == cv {
			current = rep
		} else if cv > 0 && row.Version == cv-1 {
			previous = rep
		}
	}

	return current, previous, nil
}

func (tx *gormResourceTx) FindLatestRepresentations(key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	var result struct {
		Data    internal.JsonObject
		Version uint
	}

	db := tx.getDBSession()

	query := db.Table("reporter_resources rr").
		Select("cr.data, cr.version").
		Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id")

	query = tx.buildReporterResourceKeyQuery(query, key)

	err := query.Order("cr.version DESC").Limit(1).Scan(&result).Error
	if err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return nil, wrapped
		}
		return nil, fmt.Errorf("failed to find latest representations: %w", err)
	}

	v := bizmodel.NewVersion(result.Version)
	rep, err := bizmodel.NewRepresentations(
		bizmodel.Representation(result.Data),
		&v,
		nil,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create representation: %w", err)
	}
	return rep, nil
}

// HasTransactionIdBeenProcessed checks if a transaction ID exists in either the
// reporter_representations or common_representations tables.
func (tx *gormResourceTx) HasTransactionIdBeenProcessed(transactionId bizmodel.TransactionId) (bool, error) {
	tid := transactionId.String()
	var exists bool
	err := tx.gormTx.Raw(`
	SELECT EXISTS (
		SELECT 1 FROM reporter_representations WHERE transaction_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM common_representations  WHERE transaction_id = ?
	)
	`, tid, tid).Scan(&exists).Error

	if err != nil {
		if wrapped := tx.wrapSerializationError(err); wrapped != err {
			return false, wrapped
		}
		return false, fmt.Errorf("failed to check representations for the transaction_id: %w", err)
	}
	return exists, nil
}
