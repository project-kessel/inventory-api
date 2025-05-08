package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
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
type JsonObject map[string]interface{}
type Reporter struct {
	ReporterId      string `json:"reporter_id"`
	ReporterType    string `json:"reporter_type"`
	ReporterVersion string `json:"reporter_version"`
}

func (JsonObject) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return GormDBDataType(db, field)
}

func (data JsonObject) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *JsonObject) Scan(value interface{}) error {
	return Scan(value, data)
}

func (Labels) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	return GormDBDataType(db, field)
}

func (data Labels) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *Labels) Scan(value interface{}) error {
	return Scan(value, data)
}

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
		return errors.New("failed to parse JsonObject from database")
	}
	return json.Unmarshal(b, data)
}
