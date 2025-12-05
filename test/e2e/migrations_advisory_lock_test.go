package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
)

const (
	numMigrateGoroutines    = 5
	serializationSpanFactor = 2
)

func TestMigrate_SerializesConcurrentCalls(t *testing.T) {
	enableShortMode(t)

	if db == nil {
		t.Skip("Database not available for e2e test")
	}

	logger := log.NewHelper(log.DefaultLogger)

	type executionTiming struct {
		id    int
		start time.Time
		end   time.Time
	}

	var (
		timings   []executionTiming
		timingsMu sync.Mutex
		errors    []error
		errorsMu  sync.Mutex
		wg        sync.WaitGroup
	)

	// Launch multiple goroutines that call Migrate concurrently.
	for i := 0; i < numMigrateGoroutines; i++ {
		wg.Add(1)

		go func(id int) {
			defer wg.Done()

			start := time.Now()
			err := data.Migrate(db, logger)
			end := time.Now()

			timingsMu.Lock()
			timings = append(timings, executionTiming{
				id:    id,
				start: start,
				end:   end,
			})
			timingsMu.Unlock()

			if err != nil {
				errorsMu.Lock()
				errors = append(errors, fmt.Errorf("goroutine %d: %w", id, err))
				errorsMu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if len(errors) > 0 {
		for _, err := range errors {
			t.Errorf("Migration error: %v", err)
		}
		t.FailNow()
	}

	assert.Equal(t, numMigrateGoroutines, len(timings), "all goroutines should have executed")

	for _, timing := range timings {
		t.Logf("Goroutine %d: started=%v, ended=%v, duration=%v",
			timing.id, timing.start, timing.end, timing.end.Sub(timing.start))
	}

	if len(timings) == 0 {
		t.Fatal("no timings recorded for migration calls")
	}

	firstStart := timings[0].start
	lastEnd := timings[0].end
	minDuration := timings[0].end.Sub(timings[0].start)

	for _, timing := range timings[1:] {
		if timing.start.Before(firstStart) {
			firstStart = timing.start
		}
		if timing.end.After(lastEnd) {
			lastEnd = timing.end
		}
		d := timing.end.Sub(timing.start)
		if d < minDuration {
			minDuration = d
		}
	}

	totalSpan := lastEnd.Sub(firstStart)

	if totalSpan <= minDuration*time.Duration(serializationSpanFactor) {
		t.Fatalf("advisory lock did not appear to serialize migrations: total span=%v, min duration=%v (spanFactor=%v)",
			totalSpan, minDuration, serializationSpanFactor)
	}

	t.Logf("SUCCESS: All %d migration calls were serialized by advisory lock (total span=%v, min duration=%v, spanFactor≈%.2f)",
		numMigrateGoroutines,
		totalSpan,
		minDuration,
		float64(totalSpan)/float64(minDuration))
}
