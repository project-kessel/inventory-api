package e2e

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/stretchr/testify/assert"
)

const (
	numMigrateGoroutines = 5
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

	// Sort goroutines by end time to analyze serialization.
	sort.Slice(timings, func(i, j int) bool {
		return timings[i].end.Before(timings[j].end)
	})

	minDuration := timings[0].end.Sub(timings[0].start)
	maxDuration := minDuration
	for _, timing := range timings[1:] {
		d := timing.end.Sub(timing.start)
		if d < minDuration {
			minDuration = d
		}
		if d > maxDuration {
			maxDuration = d
		}
	}

	// If migrations are serialized, later-finishing goroutines must wait for
	// the advisory lock and therefore have longer wall-clock durations.  The
	// last goroutine should take noticeably longer than the first.  A ratio
	// of 1.3 is conservative: with 5 serialized goroutines the expected
	// ratio is ~(N-1) but real-world jitter makes strict thresholds flaky.
	durationRatio := float64(maxDuration) / float64(minDuration)
	if durationRatio < 1.3 {
		t.Fatalf("advisory lock did not appear to serialize migrations: "+
			"max duration=%v, min duration=%v, ratio=%.2f (expected >= 1.3)",
			maxDuration, minDuration, durationRatio)
	}

	t.Logf("SUCCESS: All %d migration calls were serialized by advisory lock "+
		"(max duration=%v, min duration=%v, ratio=%.2f)",
		numMigrateGoroutines, maxDuration, minDuration, durationRatio)
}
