package data

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

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
		ID:            result.ResourceID,
		Type:          result.ResourceType,
		CommonVersion: result.CommonVersion,
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
	GetDB() *gorm.DB
	GetTransactionManager() usecase.TransactionManager
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

	if err := tx.Create(&dataReporterRepresentation).Error; err != nil {
		return fmt.Errorf("failed to save reporter representation: %w", err)
	}

	if err := tx.Create(&dataCommonRepresentation).Error; err != nil {
		return fmt.Errorf("failed to save common representation: %w", err)
	}

	if err := r.handleOutboxEvents(tx, resource.ResourceEvents()[0], operationType, txid); err != nil {
		return err
	}

	return nil
}

func (r *resourceRepository) handleOutboxEvents(tx *gorm.DB, resourceEvent bizmodel.ResourceReportEvent, operationType model_legacy.EventOperationType, txid string) error {
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

	err := db.Table("reporter_resources AS rr").
		Select(`
		rr2.id AS reporter_resource_id,
		rr2.representation_version,
		rr2.generation,
		rr2.tombstone,
		res.common_version,
		res.id AS resource_id,
		rr2.resource_type,
		rr2.local_resource_id,
		rr2.reporter_type,
		rr2.reporter_instance_id
	`).
		Joins(`
		JOIN reporter_resources AS rr2 ON rr2.resource_id = rr.resource_id
		JOIN resource AS res ON res.id = rr2.resource_id
	`).
		Where(`
		rr.local_resource_id = ? AND
		rr.resource_type = ? AND
		rr.reporter_type = ? AND
		rr.reporter_instance_id = ?
	`,
			key.LocalResourceId().Serialize(),
			key.ResourceType().Serialize(),
			key.ReporterType().Serialize(),
			key.ReporterInstanceId().Serialize(),
		).
		Find(&results).Error // Use Find since it returns multiple rows

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find resource by keys: %w", err)
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
