package data

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"
)

func TestMigrateToErrorsOnNilDB(t *testing.T) {
	origRunTo := migrationsRunTo
	origSession := migrationsSession
	t.Cleanup(func() {
		migrationsRunTo = origRunTo
		migrationsSession = origSession
	})

	called := false
	migrationsSession = func(db *gorm.DB) *gorm.DB {
		return db
	}

	migrationsRunTo = func(ctx context.Context, db *gorm.DB, logger *log.Helper, targetID string) error {
		called = true
		return nil
	}

	err := MigrateTo(nil, log.NewHelper(log.DefaultLogger), "20250101000000")
	if err == nil {
		t.Fatalf("expected error when db is nil")
	}
	if called {
		t.Fatalf("expected migrationsRunTo not to be called when db is nil")
	}
}

func TestMigrateToErrorsOnEmptyTargetID(t *testing.T) {
	db := &gorm.DB{}

	origRunTo := migrationsRunTo
	origSession := migrationsSession
	t.Cleanup(func() {
		migrationsRunTo = origRunTo
		migrationsSession = origSession
	})

	called := false
	migrationsSession = func(db *gorm.DB) *gorm.DB {
		return db
	}

	migrationsRunTo = func(ctx context.Context, db *gorm.DB, logger *log.Helper, targetID string) error {
		called = true
		return nil
	}

	err := MigrateTo(db, log.NewHelper(log.DefaultLogger), "")
	if err == nil {
		t.Fatalf("expected error when targetID is empty")
	}
	if called {
		t.Fatalf("expected migrationsRunTo not to be called when targetID is empty")
	}
}

func TestMigrateToDelegatesToMigrationsRunTo(t *testing.T) {
	db := &gorm.DB{}
	logger := log.NewHelper(log.DefaultLogger)
	const targetID = "20250101000000"

	var (
		calledCtx      context.Context
		calledDB       *gorm.DB
		calledLogger   *log.Helper
		calledTargetID string
	)

	origRunTo := migrationsRunTo
	origSession := migrationsSession
	t.Cleanup(func() {
		migrationsRunTo = origRunTo
		migrationsSession = origSession
	})

	migrationsSession = func(db *gorm.DB) *gorm.DB {
		return db
	}

	migrationsRunTo = func(ctx context.Context, db *gorm.DB, l *log.Helper, id string) error {
		calledCtx = ctx
		calledDB = db
		calledLogger = l
		calledTargetID = id
		return nil
	}

	if err := MigrateTo(db, logger, targetID); err != nil {
		t.Fatalf("MigrateTo returned unexpected error: %v", err)
	}

	if calledDB != db {
		t.Fatalf("expected migrationsRunTo to be called with db %p, got %p", db, calledDB)
	}
	if calledLogger != logger {
		t.Fatalf("expected migrationsRunTo to be called with provided logger")
	}
	if calledTargetID != targetID {
		t.Fatalf("expected migrationsRunTo to be called with targetID %q, got %q", targetID, calledTargetID)
	}
	if calledCtx == nil {
		t.Fatalf("expected migrationsRunTo to be called with non-nil context")
	}
}
