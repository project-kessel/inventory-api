package migrations

import (
	"strings"
	"testing"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

func TestValidateMigrationsList_EmptyList(t *testing.T) {
	err := validateMigrationsList([]*gormigrate.Migration{})
	if err != nil {
		t.Errorf("expected no error for empty list, got: %v", err)
	}
}

func TestValidateMigrationsList_SingleValidMigration(t *testing.T) {
	migrations := []*gormigrate.Migration{
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err != nil {
		t.Errorf("expected no error for valid migration, got: %v", err)
	}
}

func TestValidateMigrationsList_MultipleValidMigrations(t *testing.T) {
	migrations := []*gormigrate.Migration{
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
		{ID: "20250102120000", Migrate: func(tx *gorm.DB) error { return nil }},
		{ID: "20250103120000", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err != nil {
		t.Errorf("expected no error for valid migrations, got: %v", err)
	}
}

func TestValidateMigrationsList_EmptyID(t *testing.T) {
	migrations := []*gormigrate.Migration{
		{ID: "", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err == nil {
		t.Error("expected error for empty ID")
	}
	if !strings.Contains(err.Error(), "has empty ID") {
		t.Errorf("expected error message to mention empty ID, got: %v", err)
	}
}

func TestValidateMigrationsList_InvalidTimestampFormat(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{"not a timestamp", "invalid"},
		{"partial timestamp", "202501"},
		{"wrong format with dashes", "2025-01-01"},
		{"with colons", "20250101:12:00:00"},
		{"wrong length", "2025010112"},
		{"letters in timestamp", "2025010112000a"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			migrations := []*gormigrate.Migration{
				{ID: tc.id, Migrate: func(tx *gorm.DB) error { return nil }},
			}
			err := validateMigrationsList(migrations)
			if err == nil {
				t.Errorf("expected error for invalid timestamp format %q", tc.id)
			}
			if !strings.Contains(err.Error(), "not a valid timestamp") {
				t.Errorf("expected error message to mention invalid timestamp, got: %v", err)
			}
		})
	}
}

func TestValidateMigrationsList_OutOfOrder(t *testing.T) {
	migrations := []*gormigrate.Migration{
		{ID: "20250103120000", Migrate: func(tx *gorm.DB) error { return nil }},
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err == nil {
		t.Error("expected error for out-of-order migrations")
	}
	if !strings.Contains(err.Error(), "must be strictly later") {
		t.Errorf("expected error message to mention ordering, got: %v", err)
	}
}

func TestValidateMigrationsList_EqualTimestamps(t *testing.T) {
	migrations := []*gormigrate.Migration{
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err == nil {
		t.Error("expected error for equal timestamps (not strictly increasing)")
	}
	if !strings.Contains(err.Error(), "must be strictly later") {
		t.Errorf("expected error message to mention ordering, got: %v", err)
	}
}

func TestValidateMigrationsList_SlightlyOutOfOrder(t *testing.T) {
	// Test case where second migration is only 1 second before the first
	migrations := []*gormigrate.Migration{
		{ID: "20250101120001", Migrate: func(tx *gorm.DB) error { return nil }},
		{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
	}
	err := validateMigrationsList(migrations)
	if err == nil {
		t.Error("expected error for migrations that are out of order by just 1 second")
	}
}

func TestValidateMigrationsList_ValidEdgeCases(t *testing.T) {
	testCases := []struct {
		name       string
		migrations []*gormigrate.Migration
	}{
		{
			name: "same day different times",
			migrations: []*gormigrate.Migration{
				{ID: "20250101000000", Migrate: func(tx *gorm.DB) error { return nil }},
				{ID: "20250101235959", Migrate: func(tx *gorm.DB) error { return nil }},
			},
		},
		{
			name: "consecutive seconds",
			migrations: []*gormigrate.Migration{
				{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
				{ID: "20250101120001", Migrate: func(tx *gorm.DB) error { return nil }},
			},
		},
		{
			name: "years apart",
			migrations: []*gormigrate.Migration{
				{ID: "20200101120000", Migrate: func(tx *gorm.DB) error { return nil }},
				{ID: "20250101120000", Migrate: func(tx *gorm.DB) error { return nil }},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateMigrationsList(tc.migrations)
			if err != nil {
				t.Errorf("expected no error for %s, got: %v", tc.name, err)
			}
		})
	}
}
