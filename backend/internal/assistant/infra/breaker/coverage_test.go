package breaker

import (
	"testing"
	"time"

	"github.com/sony/gobreaker"
)

// Covers [SPEC-003: AC-004, BR-005, BR-006]

func TestCoverage_BreakerHelpers(t *testing.T) {
	t.Run("NewSettings ReadyToTrip false when Requests<5 true when ratio >=0.5 or consecutive >=3", func(t *testing.T) {
		// Arrange
		s := NewSettings(30 * time.Second)
		// Act
		if s.ReadyToTrip == nil {
			t.Fatalf("expected ReadyToTrip not nil")
		}
		// 4 requests -> false
		if s.ReadyToTrip(gobreaker.Counts{Requests: 4, TotalFailures: 4, ConsecutiveFailures: 4}) {
			t.Fatalf("expected false for <5 requests")
		}
		// 5 requests, 1 fail ratio 0.2 consecutive 1 -> false
		if s.ReadyToTrip(gobreaker.Counts{Requests: 5, TotalFailures: 1, ConsecutiveFailures: 1}) {
			t.Fatalf("expected false low ratio")
		}
		// 5 requests, 3 consecutive -> true
		if !s.ReadyToTrip(gobreaker.Counts{Requests: 5, ConsecutiveFailures: 3}) {
			t.Fatalf("expected true consecutive >=3")
		}
		// 5 requests, ratio 0.6 -> true
		if !s.ReadyToTrip(gobreaker.Counts{Requests: 10, TotalFailures: 6, ConsecutiveFailures: 1}) {
			t.Fatalf("expected true ratio >=0.5")
		}
		// OnStateChange should not panic
		if s.OnStateChange != nil {
			s.OnStateChange("gemini", gobreaker.StateClosed, gobreaker.StateOpen)
		}
		// Assert passed
	})

	t.Run("newSettings delegates to NewSettings", func(t *testing.T) {
		// Arrange
		// Act
		s := newSettings(15 * time.Second)
		// Assert
		if s.Name != "gemini" {
			t.Fatalf("expected gemini name, got %q", s.Name)
		}
		if s.Interval != 15*time.Second || s.Timeout != 15*time.Second {
			t.Fatalf("expected 15s interval/timeout, got %v %v", s.Interval, s.Timeout)
		}
		if s.MaxRequests != 1 {
			t.Fatalf("expected MaxRequests 1, got %d", s.MaxRequests)
		}
	})

	t.Run("nil breaker State IsOpen Execute Timeout Interval Breaker", func(t *testing.T) {
		// Arrange
		var b *AssistantBreaker
		var b2 = &AssistantBreaker{cb: nil, timeout: 10 * time.Second, interval: 20 * time.Second}
		// Act
		s1 := b.State()
		o1 := b.IsOpen()
		_, err1 := b.Execute(func() (any, error) { return "ok", nil })
		to1 := b.Timeout()
		iv1 := b.Interval()
		cb1 := b.Breaker()
		s2 := b2.State()
		o2 := b2.IsOpen()
		_, err2 := b2.Execute(func() (any, error) { return "ok2", nil })
		to2 := b2.Timeout()
		iv2 := b2.Interval()
		cb2 := b2.Breaker()

		// Assert
		if s1 != "closed" {
			t.Fatalf("expected closed for nil, got %q", s1)
		}
		if o1 {
			t.Fatalf("expected false IsOpen nil")
		}
		if err1 != nil {
			t.Fatalf("expected nil err, got %v", err1)
		}
		if to1 != 0 || iv1 != 0 || cb1 != nil {
			t.Fatalf("expected zero/nil for nil breaker, got %v %v %v", to1, iv1, cb1)
		}
		if s2 != "closed" {
			t.Fatalf("expected closed for nil cb, got %q", s2)
		}
		if o2 {
			t.Fatalf("expected false")
		}
		if err2 != nil {
			t.Fatalf("expected nil, got %v", err2)
		}
		if to2 != 10*time.Second || iv2 != 20*time.Second || cb2 != nil {
			t.Fatalf("expected timeout/interval preserved but breaker nil, got %v %v %v", to2, iv2, cb2)
		}
	})

	t.Run("NewAssistantBreaker and WithTimeout", func(t *testing.T) {
		// Arrange
		// Act
		b := NewAssistantBreaker()
		b2 := NewAssistantBreakerWithTimeout(5 * time.Second)
		// Assert
		if b == nil || b2 == nil {
			t.Fatalf("expected non-nil")
		}
		if b.Timeout() != DefaultTimeout {
			t.Fatalf("expected DefaultTimeout, got %v", b.Timeout())
		}
		if b2.Timeout() != 5*time.Second {
			t.Fatalf("expected 5s, got %v", b2.Timeout())
		}
		if b.Breaker() == nil || b2.Breaker() == nil {
			t.Fatalf("expected breaker cb not nil")
		}
		// also test state change
		b3 := NewAssistantBreakerWithTimeout(10 * time.Millisecond)
		for i := 0; i < 5; i++ {
			_, _ = b3.Execute(func() (any, error) { return nil, assertErr() })
		}
		if b3.State() != "open" {
			t.Fatalf("expected open")
		}
	})
}

func assertErr() error { return assertErr2 }
var assertErr2 error = gobreaker.ErrOpenState
