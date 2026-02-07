package gorm

import (
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

func TestMigrate_NilDB(t *testing.T) {
	err := Migrate(nil, log.NewHelper(log.DefaultLogger))
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
}

func TestMigrateTo_NilDB(t *testing.T) {
	err := MigrateTo(nil, log.NewHelper(log.DefaultLogger), "20250101000000")
	if err == nil {
		t.Fatal("expected error when db is nil")
	}
}

func TestMigrateTo_EmptyTargetID(t *testing.T) {
	err := MigrateTo(&gorm.DB{}, log.NewHelper(log.DefaultLogger), "")
	if err == nil {
		t.Fatal("expected error when targetID is empty")
	}
}
