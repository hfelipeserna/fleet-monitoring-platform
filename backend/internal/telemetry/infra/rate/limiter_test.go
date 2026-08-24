package rate

import (
	"context"
	"testing"
	"time"
)

func TestLimiter_Allow(t *testing.T) {
	// Covers [SPEC-001: FR-006, BR-005]
	t.Run("online allow within burst", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()

		// Act
		ok := true
		for i := 0; i < 20; i++ {
			if !l.Allow("ABC123") {
				ok = false
				break
			}
		}

		// Assert
		if !ok {
			t.Fatalf("expected 20 allows within burst")
		}
	})

	t.Run("online rate limited after burst exhausted", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()

		// Act
		for i := 0; i < 20; i++ {
			_ = l.Allow("PLATE1")
		}
		limited := !l.Allow("PLATE1")

		// Assert
		if !limited {
			t.Fatalf("expected rate limited after 20 burst")
		}
	})

	t.Run("different plates isolated", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()
		for i := 0; i < 20; i++ {
			_ = l.Allow("AAA111")
		}

		// Act
		got := l.Allow("BBB222")

		// Assert
		if !got {
			t.Fatalf("expected different plate not limited")
		}
	})

	t.Run("Stop is idempotent", func(t *testing.T) {
		// Arrange
		l := NewLimiter()

		// Act
		l.Stop()
		l.Stop()

		// Assert
		// no panic
	})
}

func TestLimiter_AllowBatch(t *testing.T) {
	// Covers [SPEC-001: FR-007, BR-005]
	t.Run("batch allow within 500 burst", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()

		// Act
		ok := l.AllowBatch("CAR001", 100)

		// Assert
		if !ok {
			t.Fatalf("expected allow for 100 within 500 burst")
		}
	})

	t.Run("batch second request within 5s limited by batchReq", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()
		_ = l.AllowBatch("CAR002", 10)

		// Act
		ok := l.AllowBatch("CAR002", 10)

		// Assert
		if ok {
			t.Fatalf("expected limited due to 1 per 5s batchReq")
		}
	})

	t.Run("batch count exceeded limits via batchCount", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()

		// Act
		ok := l.AllowBatch("CAR003", 501)

		// Assert
		if ok {
			t.Fatalf("expected limited for 501 > 500 burst")
		}
	})

	t.Run("batch zero count only checks batchReq", func(t *testing.T) {
		// Arrange
		l := NewLimiter()
		defer l.Stop()

		// Act
		ok := l.AllowBatch("CAR004", 0)

		// Assert
		if !ok {
			t.Fatalf("expected allow for n=0")
		}
		// second immediate should be limited
		ok2 := l.AllowBatch("CAR004", 0)
		if ok2 {
			t.Fatalf("expected limited on second batchReq")
		}
	})

	t.Run("AllowBatch with context background NewLimiterWithContext", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(context.Background())
		l := NewLimiterWithContext(ctx)
		defer l.Stop()
		defer cancel()

		// Act
		ok := l.Allow("XYZ789")

		// Assert
		if !ok {
			t.Fatalf("expected allow")
		}
	})

	t.Run("cleanupLoop exits on context cancel", func(t *testing.T) {
		// Arrange
		ctx, cancel := context.WithCancel(context.Background())
		l := NewLimiterWithContext(ctx)

		// Act
		cancel()
		time.Sleep(10 * time.Millisecond)
		l.Stop()

		// Assert
		// no deadlock, test passes if no panic
	})
}
