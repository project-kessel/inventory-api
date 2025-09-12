package data

import (
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/model_legacy"
)

// publishes an event to the outbox table and then deletes it
// keeping the event in the outbox table is unnecessary
// as CDC will read from the write-ahead log
func PublishOutboxEvent(tx *gorm.DB, event *model_legacy.OutboxEvent) error {
	if err := tx.Create(event).Error; err != nil {
		return err
	}
	if err := tx.Delete(event).Error; err != nil {
		return err
	}
	return nil
}
