package migrations

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestWithAdvisoryLockDoesNotDeadlockConcurrentIndex(t *testing.T) {
	dsn := os.Getenv("POSTGRES_MIGRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_MIGRATION_TEST_DSN is not set")
	}

	adminDB := openPostgresLockTestDB(t, dsn)

	suffix := strings.ReplaceAll(fmt.Sprintf("%d", time.Now().UnixNano()), "-", "")
	tableName := "migration_deadlock_" + suffix
	indexName := tableName + "_idx"

	if err := adminDB.Exec(fmt.Sprintf("CREATE TABLE %s (id bigint NOT NULL)", tableName)).Error; err != nil {
		t.Fatalf("create test table: %v", err)
	}
	t.Cleanup(func() {
		if err := adminDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName)).Error; err != nil {
			t.Errorf("drop test table: %v", err)
		}
	})

	firstPodDB := openPostgresLockTestDB(t, dsn)
	secondPodDB := openPostgresLockTestDB(t, dsn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	lockHeld := make(chan struct{})
	startIndex := make(chan struct{})
	results := make(chan error, 2)

	go func() {
		results <- WithAdvisoryLock(ctx, firstPodDB, func(tx *gorm.DB) error {
			close(lockHeld)
			<-startIndex
			return tx.Exec(fmt.Sprintf(
				"CREATE INDEX CONCURRENTLY %s ON %s (id)",
				indexName,
				tableName,
			)).Error
		})
	}()

	select {
	case <-lockHeld:
	case <-ctx.Done():
		t.Fatalf("first migration did not acquire advisory lock: %v", ctx.Err())
	}

	var secondStarted sync.WaitGroup
	secondStarted.Add(1)
	go func() {
		secondStarted.Done()
		results <- WithAdvisoryLock(ctx, secondPodDB, func(_ *gorm.DB) error {
			return nil
		})
	}()
	secondStarted.Wait()

	// Give the second migration enough time to attempt the same advisory lock
	// before the first migration begins its concurrent index build. A blocking
	// advisory-lock waiter creates the virtual-transaction dependency that can
	// deadlock CREATE INDEX CONCURRENTLY.
	time.Sleep(500 * time.Millisecond)
	close(startIndex)

	for range 2 {
		select {
		case err := <-results:
			if err != nil {
				t.Errorf("concurrent migration failed: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("concurrent migrations did not finish: %v", ctx.Err())
		}
	}
}

func openPostgresLockTestDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get PostgreSQL connection pool: %v", err)
	}
	sqlDB.SetMaxOpenConns(2)
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close PostgreSQL connection pool: %v", err)
		}
	})

	return db
}
