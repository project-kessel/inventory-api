package model_legacy

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"

	"github.com/project-kessel/inventory-api/internal"
)

type GormDbAfterMigrationHook interface {
	GormDbAfterMigration(*gorm.DB, *schema.Schema) error
}

type OperationType string

const (
	OperationTypeCreate OperationType = "CREATE"
	OperationTypeUpdate OperationType = "UPDATE"
	OperationTypeDelete OperationType = "DELETE"
)

func (o *OperationType) Scan(value interface{}) error {
	*o = OperationType(value.(string))
	return nil
}

func (o OperationType) Value() (driver.Value, error) {
	return string(o), nil
}

type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Labels []Label

// JsonObject is an alias to internal.JsonObject for backward compatibility
type JsonObject = internal.JsonObject

type Reporter struct {
	ReporterId      string `json:"reporter_id"`
	ReporterType    string `json:"reporter_type"`
	ReporterVersion string `json:"reporter_version"`
}

func (Labels) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	}
	return ""
}

func (data Labels) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *Labels) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("failed to parse Labels from database")
	}
	return json.Unmarshal(b, data)
}

// Helper functions for GORM database integration
func GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	}
	return ""
}

func Scan(value interface{}, data interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("failed to parse data from database")
	}
	return json.Unmarshal(b, data)
}
