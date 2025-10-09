package consumer

import (
	"errors"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/project-kessel/inventory-api/internal/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	. "github.com/project-kessel/inventory-api/cmd/common"
)

func TestConcurrentOffsetCommits(t *testing.T) {
	t.Run("concurrent shutdown and rebalance offset commits", func(t *testing.T) {
		tester := TestCase{}
		errs := tester.TestSetup(t)
		assert.Nil(t, errs)

		// Mock the consumer methods
		mockConsumer := &mocks.MockConsumer{}
		mockConsumer.On("IsClosed").Return(false)
		// AssignmentLost may or may not be called depending on race condition timing
		mockConsumer.On("AssignmentLost").Return(false).Maybe()

		// Set up CommitOffsets to be called at most once due to coordination
		commitCallCount := 0
		mockConsumer.On("CommitOffsets", mock.Anything).Return(
			[]kafka.TopicPartition{},
			nil,
		).Run(func(args mock.Arguments) {
			commitCallCount++
		}).Maybe() // May not be called if rebalance skips due to shutdown
		mockConsumer.On("Close").Return(nil)

		tester.inv.Consumer = mockConsumer

		// Add offsets to storage
		tester.inv.OffsetStorage = []kafka.TopicPartition{
			{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(501)},
			{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(502)},
			{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(503)},
		}

		// Create channels to coordinate the concurrent operations
		shutdownDone := make(chan error, 1)
		rebalanceDone := make(chan error, 1)

		// Start shutdown in a goroutine
		go func() {
			err := tester.inv.Shutdown()
			shutdownDone <- err
		}()

		// Start rebalance callback in another goroutine
		go func() {
			event := kafka.RevokedPartitions{
				Partitions: []kafka.TopicPartition{
					{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(10)},
				},
			}
			err := tester.inv.RebalanceCallback(nil, event)
			rebalanceDone <- err
		}()

		// Wait for both operations to complete
		shutdownErr := <-shutdownDone
		rebalanceErr := <-rebalanceDone

		// Both should complete without error
		assert.Equal(t, ErrClosed, shutdownErr)
		assert.NoError(t, rebalanceErr)

		// Verify that CommitOffsets was called at most once
		// (either by shutdown or skipped by rebalance due to coordination)
		assert.LessOrEqual(t, commitCallCount, 1, "CommitOffsets should not be called multiple times concurrently")

		mockConsumer.AssertExpectations(t)
	})
}

func TestThreadSafeOffsetStorage(t *testing.T) {
	t.Run("concurrent access to offset storage is thread-safe", func(t *testing.T) {
		tester := TestCase{}
		errs := tester.TestSetup(t)
		assert.Nil(t, errs)

		mockConsumer := &mocks.MockConsumer{}
		mockConsumer.On("CommitOffsets", mock.Anything).Return([]kafka.TopicPartition{}, nil)
		tester.inv.Consumer = mockConsumer

		// Number of concurrent goroutines
		const numGoroutines = 10
		const offsetsPerGoroutine = 10

		// Channel to coordinate goroutine completion
		done := make(chan bool, numGoroutines)

		// Start multiple goroutines that add offsets concurrently
		for i := 0; i < numGoroutines; i++ {
			go func(goroutineID int) {
				defer func() { done <- true }()

				for j := 0; j < offsetsPerGoroutine; j++ {
					offset := kafka.Offset(goroutineID*offsetsPerGoroutine + j)
					partition := kafka.TopicPartition{
						Topic:     ToPointer("test-topic"),
						Partition: int32(goroutineID % 2), // Use 2 partitions
						Offset:    offset,
					}

					// Simulate the same logic as in the consume loop
					tester.inv.offsetMutex.Lock()
					tester.inv.OffsetStorage = append(tester.inv.OffsetStorage, partition)
					tester.inv.offsetMutex.Unlock()
				}
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify that all offsets were added
		expectedOffsets := numGoroutines * offsetsPerGoroutine
		assert.Equal(t, expectedOffsets, len(tester.inv.OffsetStorage))

		// Test concurrent commit
		err := tester.inv.commitStoredOffsets()
		assert.NoError(t, err)
		assert.Equal(t, 0, len(tester.inv.OffsetStorage))

		mockConsumer.AssertExpectations(t)
	})
}

func TestCommitStoredOffsetsEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		initialOffsets []kafka.TopicPartition
		commitError    error
		expectOffsets  []kafka.TopicPartition
		expectError    bool
	}{
		{
			name:           "empty offset storage returns early",
			initialOffsets: []kafka.TopicPartition{},
			commitError:    nil,
			expectOffsets:  []kafka.TopicPartition{},
			expectError:    false,
		},
		{
			name: "commit failure restores offsets",
			initialOffsets: []kafka.TopicPartition{
				{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(100)},
				{Topic: ToPointer("test-topic"), Partition: 1, Offset: kafka.Offset(200)},
			},
			commitError: errors.New("commit failed"),
			expectOffsets: []kafka.TopicPartition{
				{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(100)},
				{Topic: ToPointer("test-topic"), Partition: 1, Offset: kafka.Offset(200)},
			},
			expectError: true,
		},
		{
			name: "successful commit clears storage",
			initialOffsets: []kafka.TopicPartition{
				{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(300)},
			},
			commitError:   nil,
			expectOffsets: []kafka.TopicPartition{},
			expectError:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup(t)
			assert.Nil(t, errs)

			mockConsumer := &mocks.MockConsumer{}
			mockConsumer.On("CommitOffsets", mock.Anything).Return(test.initialOffsets, test.commitError)
			tester.inv.Consumer = mockConsumer
			tester.inv.OffsetStorage = make([]kafka.TopicPartition, len(test.initialOffsets))
			copy(tester.inv.OffsetStorage, test.initialOffsets)

			err := tester.inv.commitStoredOffsets()

			if test.expectError {
				assert.Error(t, err)
				assert.Equal(t, test.commitError, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, len(test.expectOffsets), len(tester.inv.OffsetStorage))
			for i, expectedOffset := range test.expectOffsets {
				if i < len(tester.inv.OffsetStorage) {
					assert.Equal(t, expectedOffset.Partition, tester.inv.OffsetStorage[i].Partition)
					assert.Equal(t, expectedOffset.Offset, tester.inv.OffsetStorage[i].Offset)
				}
			}

			if len(test.initialOffsets) > 0 {
				mockConsumer.AssertExpectations(t)
			}
		})
	}
}

func TestRebalanceCallbackShutdownCoordination(t *testing.T) {
	tests := []struct {
		name               string
		shutdownInProgress bool
		hasStoredOffsets   bool
		assignmentLost     bool
		expectCommitCall   bool
		expectSkipMessage  bool
	}{
		{
			name:               "shutdown in progress skips commit",
			shutdownInProgress: true,
			hasStoredOffsets:   true,
			assignmentLost:     false,
			expectCommitCall:   false,
			expectSkipMessage:  true,
		},
		{
			name:               "no offsets to commit returns early",
			shutdownInProgress: false,
			hasStoredOffsets:   false,
			assignmentLost:     false,
			expectCommitCall:   false,
			expectSkipMessage:  false,
		},
		{
			name:               "normal rebalance commits offsets",
			shutdownInProgress: false,
			hasStoredOffsets:   true,
			assignmentLost:     false,
			expectCommitCall:   true,
			expectSkipMessage:  false,
		},
		{
			name:               "assignment lost still commits offsets",
			shutdownInProgress: false,
			hasStoredOffsets:   true,
			assignmentLost:     true,
			expectCommitCall:   true,
			expectSkipMessage:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tester := TestCase{}
			errs := tester.TestSetup(t)
			assert.Nil(t, errs)

			mockConsumer := &mocks.MockConsumer{}

			// AssignmentLost is only called if we don't return early
			if test.expectCommitCall {
				mockConsumer.On("AssignmentLost").Return(test.assignmentLost)
				mockConsumer.On("CommitOffsets", mock.Anything).Return([]kafka.TopicPartition{}, nil)
			}

			tester.inv.Consumer = mockConsumer

			// Set up initial state
			tester.inv.shutdownInProgress = test.shutdownInProgress
			if test.hasStoredOffsets {
				tester.inv.OffsetStorage = []kafka.TopicPartition{
					{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(100)},
				}
			}

			event := kafka.RevokedPartitions{
				Partitions: []kafka.TopicPartition{
					{Topic: ToPointer("test-topic"), Partition: 0, Offset: kafka.Offset(10)},
				},
			}

			err := tester.inv.RebalanceCallback(nil, event)
			assert.NoError(t, err)

			// The main test is that the function completes without error
			// and respects the coordination logic (verified through mock expectations)

			mockConsumer.AssertExpectations(t)
		})
	}
}
