package model

import (
	"fmt"
	"strings"
)

// GORM tag helper functions
// These functions help generate consistent GORM struct tags using the defined constants

// buildGORMTag creates a GORM tag string from the provided options
func buildGORMTag(options ...string) string {
	return strings.Join(options, ";")
}

// sizeTag creates a size tag for GORM
func sizeTag(size int) string {
	return fmt.Sprintf("size:%d", size)
}

// columnTag creates a column tag for GORM
func columnTag(name string) string {
	return fmt.Sprintf("column:%s", name)
}

// typeTag creates a type tag for GORM
func typeTag(dbType string) string {
	return fmt.Sprintf("type:%s", dbType)
}

// checkTag creates a check constraint tag for GORM
func checkTag(constraint string) string {
	return fmt.Sprintf("check:%s", constraint)
}

// indexTag creates an index tag for GORM
func indexTag(name string, unique bool) string {
	if unique {
		return fmt.Sprintf("index:%s,unique", name)
	}
	return fmt.Sprintf("index:%s", name)
}

// primaryKeyTag creates a primary key tag
func primaryKeyTag() string {
	return "primary_key"
}

// Common GORM tag builders for frequently used combinations

// StandardStringField creates a GORM tag for a standard string field
func StandardStringField(column string, size int) string {
	return buildGORMTag(sizeTag(size), columnTag(column))
}

// BigIntField creates a GORM tag for a bigint field with check constraint
func BigIntField(column string, checkConstraint string) string {
	return buildGORMTag(typeTag(DBTypeBigInt), columnTag(column), checkTag(checkConstraint))
}

// PrimaryKeyField creates a GORM tag for a primary key field
func PrimaryKeyField(column string, dbType string) string {
	return buildGORMTag(typeTag(dbType), columnTag(column), primaryKeyTag())
}

// UniqueIndexField creates a GORM tag for a field that's part of a unique index
func UniqueIndexField(column string, size int, indexName string) string {
	return buildGORMTag(sizeTag(size), columnTag(column), indexTag(indexName, true))
}
