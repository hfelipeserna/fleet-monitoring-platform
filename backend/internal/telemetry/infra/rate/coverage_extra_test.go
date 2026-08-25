package rate

import (
	"context"
	"testing"
	"time"
)

// Covers [SPEC-001: AC-005, BR-005]

func TestCoverage_RateCleanup(t *testing.T) {
	t.Run("NewLimiterWithContext cancelled immediately", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		// Act
		l := NewLimiterWithContext(ctx)
		// wait a bit for cleanupLoop to exit via ctx.Done
		time.Sleep(20 * time.Millisecond)
		// Assert
		if l == nil {
			t.Fatalf("expected limiter")
		}
		l.Stop()
	})

	t.Run("AllowBatch n<=0 returns true via batchReq", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()
		// Act
		ok := l.AllowBatch("plate-A", 0)
		// Assert
		if !ok {
			t.Fatalf("expected true for n<=0 when batchReq allows")
		}
		// also test negative with new plate
		ok2 := l.AllowBatch("plate-B", -1)
		if !ok2 {
			t.Fatalf("expected true negative")
		}
	})
}
