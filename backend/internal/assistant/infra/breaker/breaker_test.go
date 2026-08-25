package breaker_test

import (
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"

	"fleetmonitoring/backend/internal/assistant/infra/breaker"
)

// Covers [SPEC-003: AC-004, AC-008, BR-005, BR-006]

func TestBreaker_50pct_Open_after_3_fails(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005]
	t.Run("50% failure ratio with 5 requests and 3 fails trips open StateOpen 30s timeout", func(t *testing.T) {
		// Arrange
		cb := breaker.NewAssistantBreaker()
		if cb == nil {
			t.Fatalf("expected breaker instance, got nil")
		}

		// Act
		for i := 0; i < 2; i++ {
			_, _ = cb.Execute(func() (any, error) { return nil, nil })
		}
		for i := 0; i < 3; i++ {
			_, _ = cb.Execute(func() (any, error) { return nil, errors.New("gemini failure") })
		}

		// Assert
		if cb.State() != gobreaker.StateOpen.String() && cb.State() != "open" {
			t.Fatalf("expected breaker state open after 3/5 failures, got %q", cb.State())
		}
		if !cb.IsOpen() {
			t.Fatalf("expected IsOpen true after 50%% threshold, state %q", cb.State())
		}
		if cb.Timeout() != 30*time.Second {
			t.Fatalf("expected Timeout 30s per BR-005, got %v", cb.Timeout())
		}
		if cb.Interval() != 30*time.Second {
			t.Fatalf("expected Interval 30s per BR-005/006, got %v", cb.Interval())
		}
		_, err := cb.Execute(func() (any, error) { return nil, nil })
		if !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState when open, got %v", err)
		}
	})
}

func TestBreaker_HalfOpen_probe_success_closes(t *testing.T) {
	// Covers [SPEC-003: AC-004, AC-008, BR-005, BR-006]
	t.Run("open -> 30s -> half-open probe success -> closed", func(t *testing.T) {
		// Arrange
		cb := breaker.NewAssistantBreaker()
		for i := 0; i < 5; i++ {
			_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		}
		if cb.State() != "open" {
			t.Fatalf("precondition: expected open, got %q", cb.State())
		}
		timeout := cb.Timeout()
		if timeout != 30*time.Second {
			t.Fatalf("expected Timeout 30s, got %v", timeout)
		}

		// Act
		// Production Timeout is 30s; for fast deterministic unit test we also verify transition logic
		// via a short-lived twin breaker with same ReadyToTrip but 20ms timeout.
		fast := breaker.NewAssistantBreakerWithTimeout(20 * time.Millisecond)
		for i := 0; i < 5; i++ {
			_, _ = fast.Execute(func() (any, error) { return nil, errors.New("fail") })
		}
		time.Sleep(30 * time.Millisecond)
		_, probeErr := fast.Execute(func() (any, error) { return "probe-ok", nil })

		// Assert
		if probeErr != nil {
			t.Fatalf("expected half-open probe success, got err %v", probeErr)
		}
		if fast.State() != "closed" && fast.State() != gobreaker.StateClosed.String() {
			t.Fatalf("expected closed after successful half-open probe, got %q", fast.State())
		}
		if fast.IsOpen() {
			t.Fatalf("expected IsOpen false after closed, state %q", fast.State())
		}
		_ = cb // keep production breaker open state verified above without 30s sleep
	})
}

func TestBreaker_HalfOpen_probe_fail_reopens(t *testing.T) {
	// Covers [SPEC-003: AC-004, AC-008, BR-005, BR-006]
	t.Run("probe fail reopens breaker stays open another 30s", func(t *testing.T) {
		// Arrange
		cb := breaker.NewAssistantBreaker()
		for i := 0; i < 5; i++ {
			_, _ = cb.Execute(func() (any, error) { return nil, errors.New("fail") })
		}
		if cb.State() != "open" {
			t.Fatalf("precondition: expected open, got %q", cb.State())
		}
		if cb.Timeout() != 30*time.Second {
			t.Fatalf("expected Timeout 30s, got %v", cb.Timeout())
		}

		// Act
		fast := breaker.NewAssistantBreakerWithTimeout(20 * time.Millisecond)
		for i := 0; i < 5; i++ {
			_, _ = fast.Execute(func() (any, error) { return nil, errors.New("fail") })
		}
		time.Sleep(30 * time.Millisecond)
		_, probeErr := fast.Execute(func() (any, error) { return nil, errors.New("probe fail") })

		// Assert
		if probeErr == nil {
			t.Fatalf("expected probe failure to return error")
		}
		if fast.State() != "open" && fast.State() != gobreaker.StateOpen.String() {
			t.Fatalf("expected open after failed half-open probe, got %q", fast.State())
		}
		if !fast.IsOpen() {
			t.Fatalf("expected IsOpen true after failed probe, state %q", fast.State())
		}
		_ = cb
	})
}
