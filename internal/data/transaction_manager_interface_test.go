package data

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/usecase"
)

// TestTransactionManagerInterface tests that both GormTransactionManager and FakeTransactionManager
// properly implement the TransactionManager interface and behave correctly.
func TestTransactionManagerInterface(t *testing.T) {
	implementations := []struct {
		name                  string
		createTransactionManager func() usecase.TransactionManager
		requiresRealDB        bool
	}{
		{
			name: "GormTransactionManager",
			createTransactionManager: func() usecase.TransactionManager {
				return NewGormTransactionManager(3)
			},
			requiresRealDB: true,
		},
		{
			name: "FakeTransactionManager",
			createTransactionManager: func() usecase.TransactionManager {
				return NewFakeTransactionManager()
			},
			requiresRealDB: false,
		},
	}
	
	for _, impl := range implementations {
		t.Run(impl.name, func(t *testing.T) {
			tm := impl.createTransactionManager()
			
			var db *gorm.DB
			if impl.requiresRealDB {
				db = setupTestDatabase(t)
			} else {
				// For fake implementation, we can use a minimal DB or even nil
				db = setupTestDatabase(t) // Using real DB for consistency in testing
			}
			
			t.Run("SuccessfulTransaction", func(t *testing.T) {
				callCount := 0
				err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
					callCount++
					return nil
				})
				
				assert.NoError(t, err)
				assert.Equal(t, 1, callCount, "Transaction function should be called exactly once")
			})
			
			t.Run("FailedTransaction", func(t *testing.T) {
				expectedErr := errors.New("transaction error")
				callCount := 0
				
				err := tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
					callCount++
					return expectedErr
				})
				
				assert.Error(t, err)
				assert.Contains(t, err.Error(), expectedErr.Error())
				assert.Equal(t, 1, callCount, "Transaction function should be called exactly once for non-serialization errors")
			})
			
			t.Run("DatabaseOperations", func(t *testing.T) {
				if !impl.requiresRealDB {
					t.Skip("Skipping database operations test for fake implementation")
				}
				
				// Create a test table
				err := db.Exec("CREATE TABLE IF NOT EXISTS interface_test (id INTEGER PRIMARY KEY, name TEXT)").Error
				require.NoError(t, err)
				
				// Test insert operation through transaction manager
				err = tm.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
					return tx.Exec("INSERT INTO interface_test (name) VALUES (?)", "test_record").Error
				})
				
				assert.NoError(t, err)
				
				// Verify the record was inserted
				var count int64
				err = db.Model(&struct{}{}).Table("interface_test").Count(&count).Error
				assert.NoError(t, err)
				assert.Equal(t, int64(1), count, "Should have inserted one record")
			})
		})
	}
}

// TestFakeTransactionManagerSpecificFeatures tests features specific to the fake implementation
func TestFakeTransactionManagerSpecificFeatures(t *testing.T) {
	fake := NewFakeTransactionManager()
	db := setupTestDatabase(t)
	
	t.Run("CallCountTracking", func(t *testing.T) {
		assert.Equal(t, 0, fake.CallCount, "Initial call count should be 0")
		
		// Execute multiple transactions
		for i := 0; i < 3; i++ {
			err := fake.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
				return nil
			})
			assert.NoError(t, err)
		}
		
		assert.Equal(t, 3, fake.CallCount, "Call count should track all executions")
	})
	
	t.Run("FailureSimulation", func(t *testing.T) {
		fake.Reset()
		expectedErr := errors.New("simulated failure")
		fake.SimulateFailure(expectedErr)
		
		err := fake.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
			return nil // This won't be called due to simulated failure
		})
		
		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Equal(t, 1, fake.CallCount)
	})
	
	t.Run("RetrySimulation", func(t *testing.T) {
		fake.Reset()
		fake.SimulateRetries(2)
		
		callCount := 0
		err := fake.HandleSerializableTransaction(db, func(tx *gorm.DB) error {
			callCount++
			return nil
		})
		
		assert.NoError(t, err)
		assert.Equal(t, 1, fake.CallCount, "HandleSerializableTransaction called once")
		assert.Equal(t, 1, callCount, "Transaction function called once")
		assert.Equal(t, 3, fake.RetryAttempts, "Should track retry attempts (2 retries + 1 success)")
	})
	
	t.Run("Reset", func(t *testing.T) {
		// Set some state
		fake.SimulateFailure(errors.New("test"))
		fake.SimulateRetries(5)
		_ = fake.HandleSerializableTransaction(db, func(tx *gorm.DB) error { return nil })
		
		// Verify state has changed
		assert.NotEqual(t, 0, fake.CallCount)
		assert.True(t, fake.ShouldFail)
		
		// Reset and verify clean state
		fake.Reset()
		assert.Equal(t, 0, fake.CallCount)
		assert.False(t, fake.ShouldFail)
		assert.Nil(t, fake.FailureError)
		assert.Equal(t, 0, fake.RetryCount)
		assert.Equal(t, 0, fake.RetryAttempts)
	})
}

// TestTransactionManagerInterfaceCompliance verifies that both implementations satisfy the interface
func TestTransactionManagerInterfaceCompliance(t *testing.T) {
	var _ usecase.TransactionManager = (*GormTransactionManager)(nil)
	var _ usecase.TransactionManager = (*FakeTransactionManager)(nil)
}

// setupTestDatabase creates an in-memory SQLite database for testing
func setupTestDatabase(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	return db
}