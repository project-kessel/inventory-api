package data

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestTransactionManagerContract(t *testing.T) {
	db := setupContractTestDB(t)

	// Initialize transaction managers
	tm := NewTransactionManager(3)
	fake := NewFakeTransactionManager()

	fmt.Println("=== STARTING ENHANCED CONTRACT TESTS (Legacy vs New vs Fake) ===")

	t.Run("SuccessfulTransaction", func(t *testing.T) {
		testSuccessfulTransaction(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	t.Run("TransactionFunctionError", func(t *testing.T) {
		testTransactionFunctionError(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	t.Run("MultipleDatabaseOperations", func(t *testing.T) {
		testMultipleDatabaseOperations(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	t.Run("RollbackOnError", func(t *testing.T) {
		testRollbackOnError(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	t.Run("ErrorMessageFormat", func(t *testing.T) {
		testErrorMessageFormat(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	t.Run("FakeSpecificBehavior", func(t *testing.T) {
		testFakeSpecificBehavior(t, db, legacyHandleSerializableTransaction, tm, fake)
	})

	fmt.Println("\n=== ALL ENHANCED CONTRACT TESTS COMPLETED ===")
}

// setupContractTestDB creates an in-memory SQLite database for contract testing
func setupContractTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Auto-migrate the schema
	if err := db.AutoMigrate(&model.Resource{}, &model.ResourceHistory{}, &model.InventoryResource{}); err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func createTestResource() *model.Resource {
	id, _ := uuid.NewV7()
	return &model.Resource{
		ID:                 id,
		OrgId:              "test-org",
		ResourceType:       "host",
		WorkspaceId:        "test-workspace",
		ReporterResourceId: id.String(),
		ReporterType:       "hbi",
		ReporterInstanceId: "test-instance-" + id.String()[:8],
		ReporterVersion:    "1.0.0",
		ReporterId:         "test_reporter",
		ResourceData: map[string]any{
			"hostname": "test-host",
			"ip":       "192.168.1.1",
		},
		ConsoleHref: "/console/hosts/" + id.String(),
		ApiHref:     "/api/v1/hosts/" + id.String(),
		Reporter: model.ResourceReporter{
			Reporter: model.Reporter{
				ReporterId:      "test_reporter",
				ReporterType:    "hbi",
				ReporterVersion: "1.0.0",
			},
			LocalResourceId: id.String(),
		},
	}
}

// Legacy implementation copied from the original handleSerializableTransaction
func legacyHandleSerializableTransaction(db *gorm.DB, maxRetries int, txFunc func(tx *gorm.DB) error) error {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		tx := db.Begin()
		if tx.Error != nil {
			return fmt.Errorf("transaction failed: %w", tx.Error)
		}

		// Skip the SERIALIZABLE setting for SQLite as it doesn't support it
		// SQLite defaults to serializable isolation level anyway

		err := txFunc(tx)
		if err != nil {
			tx.Rollback()
			if legacyIsSerializationFailure(err, attempt, maxRetries) {
				continue
			}
			return fmt.Errorf("transaction failed: %w", err)
		}

		if err := tx.Commit().Error; err != nil {
			if legacyIsSerializationFailure(err, attempt, maxRetries) {
				continue
			}
			return fmt.Errorf("transaction failed: %w", err)
		}

		return nil
	}

	return fmt.Errorf("transaction failed: max retries (%d) exceeded", maxRetries)
}

func legacyIsSerializationFailure(err error, attempt, maxRetries int) bool {
	if attempt >= maxRetries {
		return false
	}

	errorStr := err.Error()
	return (errorStr == "database is locked") ||
		(errorStr == "SQLITE_BUSY") ||
		(errorStr == "database table is locked") ||
		(errorStr == "serialization failure") ||
		(errorStr == "could not serialize access due to concurrent update")
}

func testSuccessfulTransaction(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing successful transaction execution...")

	// Test with legacy implementation
	var legacyResult *model.Resource
	legacyErr := legacy(db, tm.MaxSerializationRetries, func(tx *gorm.DB) error {
		resource := createTestResource()
		if err := tx.Create(resource).Error; err != nil {
			return err
		}
		legacyResult = resource
		return nil
	})

	// Test with new implementation
	var newResult *model.Resource
	newErr := tm.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		resource := createTestResource()
		if err := tx.Create(resource).Error; err != nil {
			return err
		}
		newResult = resource
		return nil
	})

	// Test with fake implementation (no real database operations)
	var fakeResult *model.Resource
	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		resource := createTestResource()
		fakeResult = resource
		// Fake doesn't actually perform database operations
		return nil
	})

	// All should succeed
	assert.NoError(t, legacyErr)
	assert.NoError(t, newErr)
	assert.NoError(t, fakeErr)
	assert.NotNil(t, legacyResult)
	assert.NotNil(t, newResult)
	assert.NotNil(t, fakeResult)

	fmt.Println("    ✓ All three implementations successfully executed transactions")
}

func testTransactionFunctionError(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing transaction function error handling...")

	testError := errors.New("test transaction function error")

	// Test with legacy implementation
	legacyErr := legacy(db, tm.MaxSerializationRetries, func(tx *gorm.DB) error {
		return testError
	})

	// Test with new implementation
	newErr := tm.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return testError
	})

	// Test with fake implementation
	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return testError
	})

	// All should return errors with consistent patterns
	assert.Error(t, legacyErr)
	assert.Error(t, newErr)
	assert.Error(t, fakeErr)

	// Production implementations should have "transaction failed" wrapper
	assert.Contains(t, legacyErr.Error(), "transaction failed")
	assert.Contains(t, newErr.Error(), "transaction failed")
	assert.Contains(t, legacyErr.Error(), testError.Error())
	assert.Contains(t, newErr.Error(), testError.Error())

	// Fake should return the original error directly
	assert.Equal(t, testError, fakeErr)

	fmt.Println("    ✓ All implementations handled transaction function errors appropriately")
}

func testMultipleDatabaseOperations(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing multiple database operations in transaction...")

	// Test with legacy implementation
	legacyErr := legacy(db, tm.MaxSerializationRetries, func(tx *gorm.DB) error {
		for i := 0; i < 3; i++ {
			resource := createTestResource()
			resource.OrgId = fmt.Sprintf("legacy-org-%d", i)
			if err := tx.Create(resource).Error; err != nil {
				return err
			}
		}
		return nil
	})

	// Clear database for new test
	db.Exec("DELETE FROM resources")

	// Test with new implementation
	newErr := tm.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		for i := 0; i < 3; i++ {
			resource := createTestResource()
			resource.OrgId = fmt.Sprintf("new-org-%d", i)
			if err := tx.Create(resource).Error; err != nil {
				return err
			}
		}
		return nil
	})

	// Test with fake implementation (no real database operations)
	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		for i := 0; i < 3; i++ {
			resource := createTestResource()
			resource.OrgId = fmt.Sprintf("fake-org-%d", i)
			// Fake doesn't actually perform database operations
		}
		return nil
	})

	// All should succeed
	assert.NoError(t, legacyErr)
	assert.NoError(t, newErr)
	assert.NoError(t, fakeErr)

	// Verify production implementations created the expected number of resources
	var count int64
	db.Model(&model.Resource{}).Count(&count)
	assert.Equal(t, int64(3), count) // Only the new implementation's resources should remain

	fmt.Println("    ✓ All implementations handled multiple database operations correctly")
}

func testRollbackOnError(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing rollback behavior on errors...")

	// Count initial resources
	var initialCount int64
	db.Model(&model.Resource{}).Count(&initialCount)

	testError := errors.New("intentional test error")

	// Test rollback with legacy implementation
	legacyErr := legacy(db, tm.MaxSerializationRetries, func(tx *gorm.DB) error {
		resource := createTestResource()
		resource.OrgId = "rollback-test-legacy"
		if err := tx.Create(resource).Error; err != nil {
			return err
		}
		return testError
	})

	// Test rollback with new implementation
	newErr := tm.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		resource := createTestResource()
		resource.OrgId = "rollback-test-new"
		if err := tx.Create(resource).Error; err != nil {
			return err
		}
		return testError
	})

	// Test rollback with fake implementation
	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		// Fake simulates rollback behavior
		return testError
	})

	// All should return errors
	assert.Error(t, legacyErr)
	assert.Error(t, newErr)
	assert.Error(t, fakeErr)

	// Verify no resources were actually created by production implementations (rollback worked)
	var finalCount int64
	db.Model(&model.Resource{}).Count(&finalCount)
	assert.Equal(t, initialCount, finalCount)

	fmt.Println("    ✓ All implementations handled rollback behavior appropriately")
}

func testErrorMessageFormat(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing error message format consistency...")

	testError := errors.New("test error for message format")

	// Test with legacy implementation
	legacyErr := legacy(db, tm.MaxSerializationRetries, func(tx *gorm.DB) error {
		return testError
	})

	// Test with new implementation
	newErr := tm.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return testError
	})

	// Test with fake implementation
	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return testError
	})

	// All should have errors
	assert.Error(t, legacyErr)
	assert.Error(t, newErr)
	assert.Error(t, fakeErr)

	// Production implementations should have identical error message formats
	assert.Contains(t, legacyErr.Error(), "transaction failed")
	assert.Contains(t, newErr.Error(), "transaction failed")
	assert.Contains(t, legacyErr.Error(), testError.Error())
	assert.Contains(t, newErr.Error(), testError.Error())
	assert.Equal(t, legacyErr.Error(), newErr.Error())

	// Fake should return the original error directly
	assert.Equal(t, testError, fakeErr)

	fmt.Println("    ✓ All implementations return appropriately formatted error messages")
}

func testFakeSpecificBehavior(t *testing.T, db *gorm.DB, legacy func(*gorm.DB, int, func(*gorm.DB) error) error, tm *TransactionManager, fake *FakeTransactionManager) {
	fmt.Println("  Testing FakeTransactionManager specific behavior...")

	// Test configurable error behavior
	configuredError := errors.New("configured fake error")
	fake.SetNextError(configuredError)

	fakeErr := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return nil // This should be ignored due to configured error
	})

	assert.Error(t, fakeErr)
	assert.Equal(t, configuredError, fakeErr)

	// Test that error is cleared after use
	fakeErr2 := fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return nil
	})
	assert.NoError(t, fakeErr2)

	// Test call count tracking
	initialCount := fake.GetCallCount()
	fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return nil
	})
	fake.ExecuteInSerializableTransaction(db, func(tx *gorm.DB) error {
		return nil
	})
	assert.Equal(t, initialCount+2, fake.GetCallCount())

	fmt.Println("    ✓ FakeTransactionManager provides configurable behavior for advanced testing")
}
