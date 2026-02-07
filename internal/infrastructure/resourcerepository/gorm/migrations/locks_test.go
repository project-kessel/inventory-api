package migrations

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func openSQLiteForLock(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

func TestWithAdvisoryLock_SqliteRunsFn(t *testing.T) {
	db := openSQLiteForLock(t)
	ctx := context.Background()
	called := false

	err := WithAdvisoryLock(ctx, db, func(tx *gorm.DB) error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("expected function to be called")
	}
}

func TestWithAdvisoryLock_SqlitePropagatesError(t *testing.T) {
	db := openSQLiteForLock(t)
	ctx := context.Background()
	myErr := errors.New("boom")

	err := WithAdvisoryLock(ctx, db, func(tx *gorm.DB) error {
		return myErr
	})
	if !errors.Is(err, myErr) {
		t.Fatalf("expected %v, got %v", myErr, err)
	}
}
