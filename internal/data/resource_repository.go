package data

import (
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal"
	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
	datamodel "github.com/project-kessel/inventory-api/internal/data/model"
)

type FindResourceByKeysResult struct {
	ReporterResourceID    uuid.UUID `gorm:"column:reporter_resource_id"`
	RepresentationVersion uint      `gorm:"column:representation_version"`
	Generation            uint      `gorm:"column:generation"`
	Tombstone             bool      `gorm:"column:tombstone"`
	CommonVersion         uint      `gorm:"column:common_version"`
	ResourceID            uuid.UUID `gorm:"column:resource_id"`
	ResourceType          string    `gorm:"column:resource_type"`
	LocalResourceID       string    `gorm:"column:local_resource_id"`
	ReporterType          string    `gorm:"column:reporter_type"`
	ReporterInstanceID    string    `gorm:"column:reporter_instance_id"`
	ConsistencyToken      string    `gorm:"column:consistency_token"`
}

type RepresentationsByVersion struct {
	Data    internal.JsonObject `gorm:"column:data"`
	Version uint                `gorm:"column:version"`
}

// GetCurrentAndPreviousWorkspaceID extracts current and previous workspace IDs from a slice of RepresentationsByVersion
func GetCurrentAndPreviousWorkspaceID(representations []RepresentationsByVersion, currentVersion uint) (currentWorkspaceID, previousWorkspaceID string) {
	for _, repr := range representations {
		if workspaceID, exists := repr.Data["workspace_id"].(string); exists && workspaceID != "" {
			switch repr.Version {
			case currentVersion:
				currentWorkspaceID = workspaceID
			case currentVersion - 1:
				previousWorkspaceID = workspaceID
			}
		}
	}
	return currentWorkspaceID, previousWorkspaceID
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
		ID:               result.ResourceID,
		Type:             result.ResourceType,
		CommonVersion:    result.CommonVersion,
		ConsistencyToken: result.ConsistencyToken,
	}

	// Create ReporterResourceKeySnapshot
	keySnapshot := bizmodel.ReporterResourceKeySnapshot{
		LocalResourceID:    result.LocalResourceID,
		ReporterType:       result.ReporterType,
		ResourceType:       result.ResourceType,
		ReporterInstanceID: result.ReporterInstanceID,
	}

	// Create ReporterResourceSnapshot
	reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
		ID:                    result.ReporterResourceID,
		ReporterResourceKey:   keySnapshot,
		ResourceID:            result.ResourceID,
		RepresentationVersion: result.RepresentationVersion,
		Generation:            result.Generation,
		Tombstone:             result.Tombstone,
	}

	return resourceSnapshot, reporterResourceSnapshot
}

type ResourceRepository interface {
	NextResourceId() (bizmodel.ResourceId, error)
	NextReporterResourceId() (bizmodel.ReporterResourceId, error)
	Save(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error
	FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error)
	FindVersionedRepresentationsByVersion(tx *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion uint) ([]RepresentationsByVersion, error)
	GetDB() *gorm.DB
	GetTransactionManager() usecase.TransactionManager
	HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId string) (bool, error)
}

type resourceRepository struct {
	db                 *gorm.DB
	transactionManager usecase.TransactionManager
}

func NewResourceRepository(db *gorm.DB, transactionManager usecase.TransactionManager) ResourceRepository {
	return &resourceRepository{
		db:                 db,
		transactionManager: transactionManager,
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

func (r *resourceRepository) Save(tx *gorm.DB, resource bizmodel.Resource, operationType model_legacy.EventOperationType, txid string) error {
	resourceSnapshot, reporterResourceSnapshot, reporterRepresentationSnapshot, commonRepresentationSnapshot, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	dataResource := datamodel.DeserializeResourceFromSnapshot(resourceSnapshot)
	dataReporterResource := datamodel.DeserializeReporterResourceFromSnapshot(reporterResourceSnapshot)
	dataReporterRepresentation := datamodel.DeserializeReporterRepresentationFromSnapshot(reporterRepresentationSnapshot)
	dataCommonRepresentation := datamodel.DeserializeCommonRepresentationFromSnapshot(commonRepresentationSnapshot)

	if err := tx.Save(&dataResource).Error; err != nil {
		return fmt.Errorf("failed to save resource: %w", err)
	}

	if err := tx.Save(&dataReporterResource).Error; err != nil {
		return fmt.Errorf("failed to save reporter resource: %w", err)
	}

	//TODO: make these checks better, the zero value checks right now are to avoid saving zero value rows in the representation tables and causing unique constraint failures
	if dataReporterRepresentation.ReporterResourceID != uuid.Nil {
		if err := tx.Create(&dataReporterRepresentation).Error; err != nil {
			return fmt.Errorf("failed to save reporter representation: %w", err)
		}
	}

	if dataCommonRepresentation.ResourceId != uuid.Nil {
		if err := tx.Create(&dataCommonRepresentation).Error; err != nil {
			return fmt.Errorf("failed to save common representation: %w", err)
		}
	}

	var resourceEvent bizmodel.ResourceEvent
	switch operationType {
	case model_legacy.OperationTypeDeleted:
		deleteEvents := resource.ResourceDeleteEvents()
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

func (r *resourceRepository) handleOutboxEvents(tx *gorm.DB, resourceEvent bizmodel.ResourceEvent, operationType model_legacy.EventOperationType, txid string) error {
	resourceMessage, tupleMessage, err := model_legacy.NewOutboxEventsFromResourceEvent(resourceEvent, operationType, txid)
	if err != nil {
		return err
	}

	err = PublishOutboxEvent(tx, resourceMessage)
	if err != nil {
		return err
	}

	err = PublishOutboxEvent(tx, tupleMessage)
	if err != nil {
		return err
	}

	return nil
}

func (r *resourceRepository) FindResourceByKeys(tx *gorm.DB, key bizmodel.ReporterResourceKey) (*bizmodel.Resource, error) {
	var results []FindResourceByKeysResult

	// Use provided transaction or fall back to regular DB session
	db := tx
	if db == nil {
		db = r.db.Session(&gorm.Session{})
	}

	query := db.Table("reporter_resources AS rr").
		Select(`
		rr2.id AS reporter_resource_id,
		rr2.representation_version,
		rr2.generation,
		rr2.tombstone,
		res.common_version,
		res.id AS resource_id,
		res.ktn AS consistency_token,
		rr2.resource_type,
		rr2.local_resource_id,
		rr2.reporter_type,
		rr2.reporter_instance_id
	`).
		Joins(`
		JOIN reporter_resources AS rr2 ON rr2.resource_id = rr.resource_id
		JOIN resource AS res ON res.id = rr2.resource_id
	`)

	// Build WHERE conditions using case-insensitive matching to match fake repository behavior
	query = query.Where("rr.local_resource_id = ?", key.LocalResourceId().Serialize())
	query = query.Where("rr.resource_type = ?", key.ResourceType().Serialize())
	query = query.Where("rr.reporter_type = ?", key.ReporterType().Serialize())

	// Only add reporter_instance_id condition if it's not empty
	if reporterInstanceId := key.ReporterInstanceId().Serialize(); reporterInstanceId != "" {
		query = query.Where("rr.reporter_instance_id = ?", reporterInstanceId)
	}

	err := query.Find(&results).Error // Use Find since it returns multiple rows

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

func (r *resourceRepository) GetTransactionManager() usecase.TransactionManager {
	return r.transactionManager
}

// TODO this needs to be expanded to include the reporter representations
// FindVersionedRepresentationsByVersion finds the common representations by version
func (r *resourceRepository) FindVersionedRepresentationsByVersion(tx *gorm.DB, key bizmodel.ReporterResourceKey, currentVersion uint) ([]RepresentationsByVersion, error) {
	var results []RepresentationsByVersion

	// Use provided transaction or fall back to regular DB session
	db := tx
	if db == nil {
		db = r.db.Session(&gorm.Session{})
	}

	query := db.Table("reporter_resources rr").
		Select("cr.data, cr.version").
		Joins("JOIN common_representations cr ON rr.resource_id = cr.resource_id").
		Where("LOWER(rr.local_resource_id) = LOWER(?)", key.LocalResourceId().Serialize()).
		Where("LOWER(rr.resource_type) = LOWER(?)", key.ResourceType().Serialize()).
		Where("LOWER(rr.reporter_type) = LOWER(?)", key.ReporterType().Serialize()).
		Where("(cr.version = ? OR cr.version = ?)", currentVersion, currentVersion-1)

	// Only add reporter_instance_id condition if it's not empty
	if reporterInstanceId := key.ReporterInstanceId().Serialize(); reporterInstanceId != "" {
		query = query.Where("LOWER(rr.reporter_instance_id) = LOWER(?)", reporterInstanceId)
	}

	err := query.Find(&results).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find common representations by version: %w", err)
	}

	return results, nil
}

// HasTransactionIdBeenProcessed checks if a transaction ID exists in either the
// reporter_representations or common_representations tables.
// Returns true if the transaction has already been processed, false otherwise.
func (r *resourceRepository) HasTransactionIdBeenProcessed(tx *gorm.DB, transactionId string) (bool, error) {
	if transactionId == "" {
		return false, nil
	}
	// Check representations tables using lightweight EXISTS query
	var exists bool
	err := tx.Raw(`
	SELECT EXISTS (
		SELECT 1 FROM reporter_representations WHERE transaction_id = ?
	)
	OR EXISTS (
		SELECT 1 FROM common_representations  WHERE transaction_id = ?
	)
	`, transactionId, transactionId).Scan(&exists).Error

	if err != nil {
		return false, fmt.Errorf("failed to check representations for the transaction_id: %w", err)
	}
	if exists {
		return true, nil
	}
	return false, nil
}
