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

type Label struct {
	Key   string
	Value string
}

type Labels []Label
type JsonObject map[string]interface{}
type Reporter struct {
	ReporterId      string
	ReporterType    string
	ReporterVersion string
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
	switch db.Dialector.Name() {
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
