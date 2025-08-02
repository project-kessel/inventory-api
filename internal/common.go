package internal

import (
	"database/sql/driver"
	"encoding/json"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// =============================================================================
// Utility Helper Functions
// =============================================================================

// stringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

type JsonObject map[string]interface{}

func (JsonObject) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "sqlite":
		return "JSON"
	case "postgres":
		return "JSONB"
	}
	return ""
}

func (data JsonObject) Value() (driver.Value, error) {
	return json.Marshal(data)
}

func (data *JsonObject) Scan(value interface{}) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("failed to parse JsonObject from database")
	}
	return json.Unmarshal(b, data)
}

func IsClusterPolicyRelation(relationshipType string) bool {
	return relationshipType == "ispropagatedto"
}
