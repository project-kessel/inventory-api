package data

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	bizmodel "github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
	"github.com/project-kessel/inventory-api/internal/biz/usecase"
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
	dataResource, dataReporterResource, dataReporterRepresentation, dataCommonRepresentation, err := resource.Serialize()
	if err != nil {
		return fmt.Errorf("failed to serialize resource: %w", err)
	}

	if err := tx.Save(dataResource).Error; err != nil {
		return fmt.Errorf("failed to save resource: %w", err)
	}

	if err := tx.Save(dataReporterResource).Error; err != nil {
		return fmt.Errorf("failed to save reporter resource: %w", err)
	}

	if err := tx.Create(dataReporterRepresentation).Error; err != nil {
		return fmt.Errorf("failed to save reporter representation: %w", err)
	}

	if err := tx.Create(dataCommonRepresentation).Error; err != nil {
		return fmt.Errorf("failed to save common representation: %w", err)
	}

	if err := r.handleOutboxEvents(tx, resource.ResourceEvents()[0], operationType, txid); err != nil {
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
	var result FindResourceByKeysResult

	//TODO: Fix query to do a self join on reporter_resource and return all reporter_resources with the given resource_id
	err := tx.Table("reporter_resources AS rr").
		Select(`
			rr.id AS reporter_resource_id,
			rr.representation_version,
			rr.generation,
			rr.tombstone,
			res.common_version,
			res.id AS resource_id,
			rr.resource_type,
			rr.local_resource_id,
			rr.reporter_type,
			rr.reporter_instance_id
		`).
		Joins("JOIN resource AS res ON res.id = rr.resource_id").
		Where(`
			rr.local_resource_id = ? AND
			rr.resource_type = ? AND
			rr.reporter_type = ? AND
			rr.reporter_instance_id = ?`,
			key.LocalResourceId(), key.ResourceType(), key.ReporterType(), key.ReporterInstanceId()).
		Take(&result).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find resource by keys: %w", err)
	}

	// Create snapshots from query result
	resourceSnapshot := bizmodel.ResourceSnapshot{
		ID:               result.ResourceID,
		Type:             result.ResourceType,
		CommonVersion:    result.CommonVersion,
		ConsistencyToken: "",
		CreatedAt:        time.Now(), // Placeholder - would need proper timestamps from DB
		UpdatedAt:        time.Now(), // Placeholder
	}

	reporterResourceSnapshot := bizmodel.ReporterResourceSnapshot{
		ID: result.ReporterResourceID,
		ReporterResourceKey: bizmodel.ReporterResourceKeySnapshot{
			LocalResourceID:    result.LocalResourceID,
			ReporterType:       result.ReporterType,
			ResourceType:       result.ResourceType,
			ReporterInstanceID: result.ReporterInstanceID,
		},
		ResourceID:            result.ResourceID,
		APIHref:               "redhat.com", // Placeholder - would come from DB
		ConsoleHref:           "",           // Placeholder
		RepresentationVersion: result.RepresentationVersion,
		Generation:            result.Generation,
		Tombstone:             result.Tombstone,
		CreatedAt:             time.Now(), // Placeholder
		UpdatedAt:             time.Now(), // Placeholder
	}

	// Create placeholder representation snapshots - in real implementation these would come from separate queries
	reporterRepresentationSnapshot := bizmodel.ReporterRepresentationSnapshot{
		Representation: bizmodel.RepresentationSnapshot{
			Data: map[string]interface{}{}, // Would come from separate query
		},
		ReporterResourceID: result.ReporterResourceID.String(),
		Version:            result.RepresentationVersion,
		Generation:         result.Generation,
		CommonVersion:      result.CommonVersion,
		Tombstone:          result.Tombstone,
		CreatedAt:          time.Now(),
	}

	commonRepresentationSnapshot := bizmodel.CommonRepresentationSnapshot{
		Representation: bizmodel.RepresentationSnapshot{
			Data: map[string]interface{}{}, // Would come from separate query
		},
		ResourceId:                 result.ResourceID,
		Version:                    result.CommonVersion,
		ReportedByReporterType:     result.ReporterType,
		ReportedByReporterInstance: result.ReporterInstanceID,
		CreatedAt:                  time.Now(),
	}

	// Deserialize using snapshot-based approach
	resource := bizmodel.DeserializeResource(
		resourceSnapshot,
		reporterResourceSnapshot,
		reporterRepresentationSnapshot,
		commonRepresentationSnapshot,
	)

	return &resource, nil
}

func (r *resourceRepository) GetDB() *gorm.DB {
	return r.db
}

func (r *resourceRepository) GetTransactionManager() usecase.TransactionManager {
	return r.transactionManager
}
