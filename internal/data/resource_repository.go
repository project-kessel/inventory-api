package data

import (
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
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
	// Create ResourceSnapshot
	resourceSnapshot := bizmodel.ResourceSnapshot{
		ID:                result.ResourceID,
		Type:              result.ResourceType,
		CommonVersion:     result.CommonVersion,
		LastCommonVersion: result.LastCommonVersion,
		ConsistencyToken:  result.ConsistencyToken,
		CreatedAt:         result.CreatedAt,
		UpdatedAt:         result.UpdatedAt,
	}

	// Create ReporterResourceKeySnapshot
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

type resourceRepository struct {
	db                 *gorm.DB
	transactionManager bizmodel.TransactionManager
	outboxPublisher    OutboxPublisher
}

func NewResourceRepository(db *gorm.DB, transactionManager bizmodel.TransactionManager, outboxPublisher OutboxPublisher) bizmodel.ResourceRepository {
	if outboxPublisher == nil {
		outboxPublisher = publishNoOpOutboxEvent
	}
	return &resourceRepository{
		db:                 db,
		transactionManager: transactionManager,
		outboxPublisher:    outboxPublisher,
	}
}

func (r *resourceRepository) NextResourceId() (bizmodel.ResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ResourceId{}, err
	}

	return bizmodel.NewResourceId(uuidV7)
}

func (r *resourceRepository) NextReporterResourceId() (bizmodel.ReporterResourceId, error) {
	uuidV7, err := uuid.NewV7()
	if err != nil {
		return bizmodel.ReporterResourceId{}, err
	}

	return bizmodel.NewReporterResourceId(uuidV7)
}

func (r *resourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	dataResource := datamodel.DeserializeResourceFromSnapshot(resourceSnapshot)
	dataReporterResource := datamodel.DeserializeReporterResourceFromSnapshot(reporterResourceSnapshot)

	if err := tx.Save(&dataResource).Error; err != nil {
		return fmt.Errorf("failed to save resource: %w", err)
	}

	if err := tx.Save(&dataReporterResource).Error; err != nil {
		return fmt.Errorf("failed to save reporter resource: %w", err)
	}

	if reporterRepresentationSnapshot != nil {
		dataReporterRepresentation := datamodel.DeserializeReporterRepresentationFromSnapshot(*reporterRepresentationSnapshot)
		if err := tx.Create(&dataReporterRepresentation).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errors.BadRequest(bizmodel.ReasonNonUniqueTransactionID, err.Error()).WithCause(err)
			}
			return fmt.Errorf("failed to save reporter representation: %w", err)
		}
	}

	if commonRepresentationSnapshot != nil {
		dataCommonRepresentation := datamodel.DeserializeCommonRepresentationFromSnapshot(*commonRepresentationSnapshot)
		if err := tx.Create(&dataCommonRepresentation).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errors.BadRequest(bizmodel.ReasonNonUniqueTransactionID, err.Error()).WithCause(err)
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
			// No delete events to process (e.g., resource was already tombstoned)
			return nil
		}
		resourceEvent = deleteEvents[0]
	default:
		resourceEvent = resource.ResourceReportEvents()[0]
	}
	if err := r.handleOutboxEvents(tx, resourceEvent, operationType, txid); err != nil {
		return err
	}

	return nil
}

func (r *resourceRepository) handleOutboxEvents(tx *gorm.DB, resourceEvent bizmodel.ResourceEvent, operationType bizmodel.EventOperationType, txid bizmodel.TransactionId) error {
	resourceMessage, tupleMessage, err := model_legacy.NewOutboxEventsFromResourceEvent(resourceEvent, operationType, txid)
	if err != nil {
		return err
	}

	err = r.outboxPublisher(tx, resourceMessage)
	if err != nil {
		return err
	}

	err = r.outboxPublisher(tx, tupleMessage)
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceRepository) getDBSession(tx *gorm.DB) *gorm.DB {
	if tx == nil {
		return r.db.Session(&gorm.Session{})
	}
	return tx
}

func (r *resourceRepository) buildReporterResourceKeyQuery(db *gorm.DB, key bizmodel.ReporterResourceKey) *gorm.DB {
	query := db.
		Where("rr.local_resource_id = ?", key.LocalResourceId().Serialize()).
		Where("rr.resource_type = ?", key.ResourceType().Serialize()).
		Where("rr.reporter_type = ?", key.ReporterType().Serialize())

	if reporterInstanceId := key.ReporterInstanceId().Serialize(); reporterInstanceId != "" {
		query = query.Where("rr.reporter_instance_id = ?", reporterInstanceId)
	}

	return query
}

func (r *resourceRepository) FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	var results []FindResourceByKeysResult

	db := r.getDBSession(tx)

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
	// This ensures results[0] (used as the primary resource snapshot) is the same
	// "latest" row the fake selects. We do not LIMIT 1 here because all rr2 rows
	// are intentionally collected as reporter resource snapshots.
	err := r.buildReporterResourceKeyQuery(query, key).
		Order("rr2.tombstone ASC, rr2.representation_version DESC, rr2.generation DESC").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to find resource by keys: %w", err)
	}

	if len(results) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	resourceSnapshot, reporterResourceSnapshots := ToSnapshotsFromResults(results)
	resource := bizmodel.DeserializeResource(resourceSnapshot, reporterResourceSnapshots, nil, nil)

	return resource, nil
}

func (r *resourceRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *resourceRepository) GetTransactionManager() bizmodel.TransactionManager {
	return r.transactionManager
}

func (r *resourceRepository) FindCurrentAndPreviousVersionedRepresentations(
	tx *gorm.DB,
	key bizmodel.ReporterResourceKey,
	currentCommonVersion *bizmodel.Version,
	currentReporterVersion *bizmodel.Version,
	operationType bizmodel.EventOperationType,
) (*bizmodel.Representations, *bizmodel.Representations, error) {
	// Guard against both versions being nil (should not happen per TupleEvent invariant)
	if currentCommonVersion == nil && currentReporterVersion == nil {
		return nil, nil, fmt.Errorf("at least one version must be provided")
	}

	db := r.getDBSession(tx)

	// Determine which streams advanced and fetch current/previous for each
	var currentCommon, previousCommon bizmodel.Representation
	var currentCommonVer, previousCommonVer *bizmodel.Version

	var currentReporter, previousReporter bizmodel.Representation
	var currentReporterVer, previousReporterVer *bizmodel.Version

	// Fetch common stream - only if version is provided (stream advanced)
	if currentCommonVersion != nil {
		cv := currentCommonVersion.Uint()
		currentCommon, currentCommonVer = r.fetchCommonRepresentation(db, key, cv)
		if operationType.OperationType() != bizmodel.OperationTypeCreated && cv > 0 {
			previousCommon, previousCommonVer = r.fetchCommonRepresentation(db, key, cv-1)
		}
	}
	// else: common stream didn't advance - leave nil (don't fetch)

	// Fetch reporter stream - only if version is provided (stream advanced)
	if currentReporterVersion != nil {
		rv := currentReporterVersion.Uint()
		currentReporter, currentReporterVer = r.fetchReporterRepresentation(db, key, rv)
		if operationType.OperationType() != bizmodel.OperationTypeCreated && rv > 0 {
			// Fetch immediately preceding reporter row (by version DESC, generation DESC)
			previousReporter, previousReporterVer = r.fetchPreviousReporterRepresentation(db, key, rv)
		}
	}
	// else: reporter stream didn't advance - leave nil (don't fetch)

	// Build current and previous Representations
	var current, previous *bizmodel.Representations
	var err error

	current, err = bizmodel.NewRepresentations(currentCommon, currentCommonVer, currentReporter, currentReporterVer)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create current representation: %w", err)
	}

	// Only build previous if at least one stream has data
	if len(previousCommon) > 0 || len(previousReporter) > 0 {
		previous, err = bizmodel.NewRepresentations(previousCommon, previousCommonVer, previousReporter, previousReporterVer)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create previous representation: %w", err)
		}
	}

	return current, previous, nil
}

// fetchCommonRepresentation fetches a common representation at a specific version
func (r *resourceRepository) fetchCommonRepresentation(db *gorm.DB, key bizmodel.ReporterResourceKey, version uint) (bizmodel.Representation, *bizmodel.Version) {
	type commonRepresentationRow struct {
		Data    internal.JsonObject
		Version uint
	}

	var result commonRepresentationRow
	query := db.Table("reporter_resources rr").
		Select("cr.data, cr.version").
		Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id")

	query = r.buildReporterResourceKeyQuery(query, key)
	query = query.Where("cr.version = ?", version)

	err := query.Limit(1).Scan(&result).Error
	if err != nil || len(result.Data) == 0 {
		return nil, nil
	}

	v := bizmodel.NewVersion(result.Version)
	return bizmodel.Representation(result.Data), &v
}

// fetchLatestCommonRepresentation fetches the latest common representation
func (r *resourceRepository) fetchLatestCommonRepresentation(db *gorm.DB, key bizmodel.ReporterResourceKey) (bizmodel.Representation, *bizmodel.Version) {
	type commonRepresentationRow struct {
		Data    internal.JsonObject
		Version uint
	}

	var result commonRepresentationRow
	query := db.Table("reporter_resources rr").
		Select("cr.data, cr.version").
		Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id")

	query = r.buildReporterResourceKeyQuery(query, key)

	err := query.Order("cr.version DESC").Limit(1).Scan(&result).Error
	if err != nil || len(result.Data) == 0 {
		return nil, nil
	}

	v := bizmodel.NewVersion(result.Version)
	return bizmodel.Representation(result.Data), &v
}

// fetchReporterRepresentation fetches a reporter representation at a specific version
func (r *resourceRepository) fetchReporterRepresentation(db *gorm.DB, key bizmodel.ReporterResourceKey, version uint) (bizmodel.Representation, *bizmodel.Version) {
	type reporterRepresentationRow struct {
		Data    internal.JsonObject
		Version uint
	}

	var result reporterRepresentationRow
	query := db.Table("reporter_resources rr").
		Select("rrep.data, rrep.version").
		Joins("JOIN reporter_representations rrep ON rr.id = rrep.reporter_resource_id")

	query = r.buildReporterResourceKeyQuery(query, key)
	query = query.Where("rrep.version = ?", version)

	err := query.Order("rrep.generation DESC").Limit(1).Scan(&result).Error
	if err != nil || len(result.Data) == 0 {
		return nil, nil
	}

	v := bizmodel.NewVersion(result.Version)
	return bizmodel.Representation(result.Data), &v
}

// fetchPreviousReporterRepresentation fetches the reporter representation immediately before the given version
func (r *resourceRepository) fetchPreviousReporterRepresentation(db *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion uint) (bizmodel.Representation, *bizmodel.Version) {
	type reporterRepresentationRow struct {
		Data    internal.JsonObject
		Version uint
	}

	var result reporterRepresentationRow
	query := db.Table("reporter_resources rr").
		Select("rrep.data, rrep.version").
		Joins("JOIN reporter_representations rrep ON rr.id = rrep.reporter_resource_id")

	query = r.buildReporterResourceKeyQuery(query, key)
	query = query.Where("rrep.version < ?", currentVersion)

	err := query.Order("rrep.version DESC, rrep.generation DESC").Limit(1).Scan(&result).Error
	if err != nil || len(result.Data) == 0 {
		return nil, nil
	}

	v := bizmodel.NewVersion(result.Version)
	return bizmodel.Representation(result.Data), &v
}

// fetchLatestReporterRepresentation fetches the latest reporter representation
func (r *resourceRepository) fetchLatestReporterRepresentation(db *gorm.DB, key bizmodel.ReporterResourceKey) (bizmodel.Representation, *bizmodel.Version) {
	type reporterRepresentationRow struct {
		Data    internal.JsonObject
		Version uint
	}

	var result reporterRepresentationRow
	query := db.Table("reporter_resources rr").
		Select("rrep.data, rrep.version").
		Joins("JOIN reporter_representations rrep ON rr.id = rrep.reporter_resource_id")

	query = r.buildReporterResourceKeyQuery(query, key)

	err := query.Order("rrep.version DESC, rrep.generation DESC").Limit(1).Scan(&result).Error
	if err != nil || len(result.Data) == 0 {
		return nil, nil
	}

	v := bizmodel.NewVersion(result.Version)
	return bizmodel.Representation(result.Data), &v
}

func (r *resourceRepository) FindLatestRepresentations(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Representations, error) {
	db := r.getDBSession(tx)

	// Fetch latest from both streams
	commonData, commonVersion := r.fetchLatestCommonRepresentation(db, key)
	reporterData, reporterVersion := r.fetchLatestReporterRepresentation(db, key)

	// Build representation from whatever streams exist
	rep, err := bizmodel.NewRepresentations(commonData, commonVersion, reporterData, reporterVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to create representation: %w", err)
	}
	return rep, nil
}

// HasTransactionIdBeenProcessed checks if a transaction ID exists in either the
// reporter_representations or common_representations tables.
// Returns true if the transaction has already been processed, false otherwise.
func (r *resourceRepository) HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId bizmodel.TransactionId) (bool, error) {
	tid := transactionId.String()
	var exists bool
	err := tx.Raw(`
	SELECT EXISTS (
		SELECT 1 FROM reporter_representations WHERE transaction_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM common_representations  WHERE transaction_id = ?
	)
	`, tid, tid).Scan(&exists).Error

	if err != nil {
		return false, fmt.Errorf("failed to check representations for the transaction_id: %w", err)
	}
	if exists {
		return true, nil
	}
	return false, nil
}
