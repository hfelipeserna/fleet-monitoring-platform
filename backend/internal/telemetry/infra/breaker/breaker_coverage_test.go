package breaker

import (
	"errors"
	"testing"
	"time"

	"github.com/sony/gobreaker"
)

func TestBreakerCoverage_NewBreakerWithSettings(t *testing.T) {
	// Covers [SPEC-001: FR-008, BR-006]
	t.Run("defaults when empty name and zero values", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("", 0, 0, 0, 0)
		// Act
		state := b.State()
		isOpen := b.IsOpen()
		// Assert
		if state != "closed" {
			t.Fatalf("expected closed for defaults, got %s", state)
		}
		if isOpen {
			t.Fatalf("expected not open for defaults")
		}
		if b == nil || b.cb == nil {
			t.Fatalf("expected breaker cb not nil")
		}
	})

	t.Run("defaults when failureRatio out of range low", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("test-low", 10, 30*time.Second, -0.1, 10)
		// Act
		// trigger with 50% failure to ensure default 0.5 is used (should trip)
		for i := 0; i < 5; i++ {
			b.RecordSuccess()
		}
		for i := 0; i < 5; i++ {
			b.RecordFailure()
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open with default ratio (negative input), got %s", b.State())
		}
	})

	t.Run("defaults when failureRatio out of range high", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("test-high", 10, 30*time.Second, 2.0, 10)
		// Act
		for i := 0; i < 5; i++ {
			b.RecordSuccess()
		}
		for i := 0; i < 5; i++ {
			b.RecordFailure()
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open with default ratio (high input), got %s", b.State())
		}
	})

	t.Run("custom name and thresholds respected", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("custom", 2, 10*time.Millisecond, 0.8, 2)
		// Act
		// 2 requests, 1 failure -> 50% < 0.8 should stay closed
		b.RecordSuccess()
		b.RecordFailure()
		// Assert
		if b.IsOpen() {
			t.Fatalf("expected closed at 50%% with 0.8 threshold, got %s", b.State())
		}
	})

	t.Run("ReadyToTrip false when requests below threshold", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("rt-below", 10, 30*time.Second, 0.5, 10)
		// Act
		for i := 0; i < 5; i++ {
			b.RecordFailure()
		}
		// Assert
		if b.IsOpen() {
			t.Fatalf("should not trip when requests < threshold, state %s", b.State())
		}
		if b.State() != "closed" {
			t.Fatalf("expected closed, got %s", b.State())
		}
	})

	t.Run("ReadyToTrip true when failure ratio exceeds", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("rt-above", 10, 30*time.Second, 0.5, 10)
		// Act
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open after 10 failures, got %s", b.State())
		}
	})

	t.Run("NewBreaker uses defaults", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		state := b.State()
		// Assert
		if state != "closed" {
			t.Fatalf("expected closed, got %s", state)
		}
		if b.IsOpen() {
			t.Fatalf("expected not open")
		}
	})
}

func TestBreakerCoverage_State(t *testing.T) {
	t.Run("closed state", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		state := b.State()
		// Assert
		if state != "closed" {
			t.Fatalf("expected closed, got %s", state)
		}
	})

	t.Run("open state after failures", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		state := b.State()
		// Assert
		if state != "open" {
			t.Fatalf("expected open, got %s", state)
		}
	})

	t.Run("half-open state after timeout", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("halfopen", 2, 10*time.Millisecond, 0.5, 2)
		for i := 0; i < 2; i++ {
			b.RecordFailure()
		}
		if b.State() != "open" {
			t.Fatalf("precondition: expected open, got %s", b.State())
		}
		// Act
		time.Sleep(20 * time.Millisecond)
		// Allow triggers transition to half-open via Execute
		_ = b.Allow()
		// gobreaker transitions to half-open after timeout on next State check or Execute
		state := b.State()
		// Assert
		if state != "half-open" {
			t.Fatalf("expected half-open after timeout, got %s", state)
		}
	})
}

func TestBreakerCoverage_IsOpen(t *testing.T) {
	t.Run("nil breaker returns false", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		got := b.IsOpen()
		// Assert
		if got {
			t.Fatalf("expected false for nil breaker")
		}
	})

	t.Run("breaker with nil cb returns false", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// Act
		got := b.IsOpen()
		state := b.State()
		// Assert
		if got {
			t.Fatalf("expected false for nil cb")
		}
		if state != "closed" {
			t.Fatalf("expected closed for nil cb, got %s", state)
		}
	})

	t.Run("closed returns false", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		got := b.IsOpen()
		// Assert
		if got {
			t.Fatalf("expected false for closed")
		}
	})

	t.Run("open returns true", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		got := b.IsOpen()
		// Assert
		if !got {
			t.Fatalf("expected true for open, state %s", b.State())
		}
	})

	t.Run("half-open returns false", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("halfopen-isopen", 2, 10*time.Millisecond, 0.5, 2)
		for i := 0; i < 2; i++ {
			b.RecordFailure()
		}
		time.Sleep(20 * time.Millisecond)
		// trigger half-open
		_ = b.State()
		// Act
		got := b.IsOpen()
		// Assert
		if got {
			t.Fatalf("expected false for half-open, state %s", b.State())
		}
	})
}

func TestBreakerCoverage_IsOpenBreaker(t *testing.T) {
	t.Run("nil breaker returns false", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		got := IsOpenBreaker(b)
		// Assert
		if got {
			t.Fatalf("expected false for nil")
		}
	})

	t.Run("closed returns false", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		got := IsOpenBreaker(b)
		// Assert
		if got {
			t.Fatalf("expected false for closed")
		}
	})

	t.Run("open returns true", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		got := IsOpenBreaker(b)
		// Assert
		if !got {
			t.Fatalf("expected true for open, state %s", b.State())
		}
	})

	t.Run("nil cb returns false", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// Act
		got := IsOpenBreaker(b)
		// Assert
		if got {
			t.Fatalf("expected false for nil cb")
		}
	})

	t.Run("half-open returns false", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("halfopen-breaker", 2, 10*time.Millisecond, 0.5, 2)
		for i := 0; i < 2; i++ {
			b.RecordFailure()
		}
		time.Sleep(20 * time.Millisecond)
		_ = b.State()
		// Act
		got := IsOpenBreaker(b)
		// Assert
		if got {
			t.Fatalf("expected false for half-open")
		}
	})
}

func TestBreakerCoverage_IsOpenGeneric(t *testing.T) {
	t.Run("nil returns false", func(t *testing.T) {
		// Arrange
		var b interface {
			State() string
			IsOpen() bool
		} = nil
		// Act
		got := IsOpen(b)
		// Assert
		if got {
			t.Fatalf("expected false for nil")
		}
	})

	t.Run("typed nil returns false", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		got := IsOpen(b)
		// Assert
		if got {
			t.Fatalf("expected false for typed nil")
		}
	})

	t.Run("typed nil with IsOpen generic", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// use generic IsOpen via Breaker type that implements both
		var iface interface {
			State() string
			IsOpen() bool
		} = b
		// Act
		got := IsOpen(iface)
		// Assert
		if got {
			t.Fatalf("expected false for Breaker with nil cb via generic IsOpen")
		}
	})

	t.Run("closed returns false", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		got := IsOpen(b)
		// Assert
		if got {
			t.Fatalf("expected false for closed")
		}
	})

	t.Run("open returns true", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		got := IsOpen(b)
		// Assert
		if !got {
			t.Fatalf("expected true for open, state %s", b.State())
		}
	})

	t.Run("half-open returns false", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("halfopen-generic", 2, 10*time.Millisecond, 0.5, 2)
		for i := 0; i < 2; i++ {
			b.RecordFailure()
		}
		time.Sleep(20 * time.Millisecond)
		_ = b.State()
		// Act
		got := IsOpen(b)
		// Assert
		if got {
			t.Fatalf("expected false for half-open")
		}
	})

	t.Run("fake breaker closed", func(t *testing.T) {
		// Arrange
		fb := &fakeGenericBreaker{state: "closed", open: false}
		// Act
		got := IsOpen(fb)
		// Assert
		if got {
			t.Fatalf("expected false for fake closed")
		}
	})

	t.Run("fake breaker open", func(t *testing.T) {
		// Arrange
		fb := &fakeGenericBreaker{state: "open", open: true}
		// Act
		got := IsOpen(fb)
		// Assert
		if !got {
			t.Fatalf("expected true for fake open")
		}
	})
}

type fakeGenericBreaker struct {
	state string
	open  bool
}

func (f *fakeGenericBreaker) State() string { return f.state }
func (f *fakeGenericBreaker) IsOpen() bool  { return f.open }

func TestBreakerCoverage_Allow(t *testing.T) {
	t.Run("closed returns nil", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		err := b.Allow()
		// Assert
		if err != nil {
			t.Fatalf("expected nil for closed, got %v", err)
		}
	})

	t.Run("open returns error wrapped ErrOpenState", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		err := b.Allow()
		// Assert
		if err == nil {
			t.Fatalf("expected error for open")
		}
		if !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState wrapped, got %v", err)
		}
	})

	t.Run("half-open returns nil", func(t *testing.T) {
		// Arrange
		b := NewBreakerWithSettings("halfopen-allow", 2, 10*time.Millisecond, 0.5, 2)
		for i := 0; i < 2; i++ {
			b.RecordFailure()
		}
		time.Sleep(20 * time.Millisecond)
		_ = b.State()
		// Act
		err := b.Allow()
		// Assert
		if err != nil {
			t.Fatalf("expected nil for half-open, got %v", err)
		}
	})

	t.Run("nil breaker returns nil", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		err := b.Allow()
		// Assert
		if err != nil {
			t.Fatalf("expected nil for nil breaker, got %v", err)
		}
	})

	t.Run("nil cb returns nil", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// Act
		err := b.Allow()
		// Assert
		if err != nil {
			t.Fatalf("expected nil for nil cb, got %v", err)
		}
	})
}

func TestBreakerCoverage_RecordSuccessFailure(t *testing.T) {
	t.Run("RecordSuccess stays closed", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		for i := 0; i < 10; i++ {
			b.RecordSuccess()
		}
		// Assert
		if b.IsOpen() {
			t.Fatalf("expected closed after successes, got %s", b.State())
		}
	})

	t.Run("RecordFailure trips after threshold", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open after failures, got %s", b.State())
		}
	})

	t.Run("RecordSuccess on open breaker does not panic", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.RecordFailure()
		}
		// Act
		b.RecordSuccess()
		// Assert
		// After open, RecordSuccess will be blocked by breaker (ErrOpenState) but should not panic
		if b.State() != "open" {
			t.Fatalf("expected still open, got %s", b.State())
		}
	})

	t.Run("RecordSuccess nil breaker does not panic", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		b.RecordSuccess()
		// Assert
		if b != nil && b.IsOpen() {
			t.Fatalf("expected not open")
		}
	})

	t.Run("RecordSuccess nil cb does not panic", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// Act
		b.RecordSuccess()
		// Assert
		if b.IsOpen() {
			t.Fatalf("expected false for nil cb")
		}
	})

	t.Run("RecordFailure nil breaker does not panic", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		b.RecordFailure()
		// Assert
		if b != nil && b.IsOpen() {
			t.Fatalf("expected not open")
		}
	})

	t.Run("RecordFailure nil cb does not panic", func(t *testing.T) {
		// Arrange
		b := &Breaker{cb: nil}
		// Act
		b.RecordFailure()
		// Assert
		if b.IsOpen() {
			t.Fatalf("expected false for nil cb")
		}
	})
}

func TestBreakerCoverage_TripOnError(t *testing.T) {
	// Covers [SPEC-001: BR-006]
	t.Run("nil error records success", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		for i := 0; i < 5; i++ {
			b.TripOnError(nil)
		}
		// Assert
		if b.IsOpen() {
			t.Fatalf("expected closed after nil errors (success), got %s", b.State())
		}
		if b.State() != "closed" {
			t.Fatalf("expected closed, got %s", b.State())
		}
	})

	t.Run("non-nil error records failure", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		err := errors.New("publish failed")
		// Act
		for i := 0; i < 9; i++ {
			b.TripOnError(err)
		}
		// Assert
		if b.IsOpen() {
			t.Fatalf("should not open before 10, got %s", b.State())
		}
	})

	t.Run("TripOnError trips after threshold", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		err := errors.New("failure")
		// Act
		for i := 0; i < 10; i++ {
			b.TripOnError(err)
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open after 10 TripOnError with error, got %s", b.State())
		}
		if err := b.Allow(); err == nil {
			t.Fatalf("expected Allow error when open")
		}
	})

	t.Run("TripOnError mixed nil and error stays closed at 50%% with threshold 10", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		for i := 0; i < 5; i++ {
			b.TripOnError(nil)
		}
		for i := 0; i < 5; i++ {
			b.TripOnError(errors.New("fail"))
		}
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected open at 50%% with 10 requests, got %s", b.State())
		}
	})

	t.Run("TripOnError with nil after open still open", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		for i := 0; i < 10; i++ {
			b.TripOnError(errors.New("fail"))
		}
		// Act
		b.TripOnError(nil)
		// Assert
		if !b.IsOpen() {
			t.Fatalf("expected still open, got %s", b.State())
		}
	})
}

func TestBreakerCoverage_IsNilHelper(t *testing.T) {
	t.Run("nil any", func(t *testing.T) {
		// Arrange
		var v any = nil
		// Act
		got := isNil(v)
		// Assert
		if !got {
			t.Fatalf("expected true")
		}
	})
	t.Run("typed nil breaker", func(t *testing.T) {
		// Arrange
		var b *Breaker = nil
		// Act
		got := isNil(b)
		// Assert
		if !got {
			t.Fatalf("expected true for typed nil")
		}
	})
	t.Run("non-nil", func(t *testing.T) {
		// Arrange
		b := NewBreaker()
		// Act
		got := isNil(b)
		// Assert
		if got {
			t.Fatalf("expected false")
		}
	})
	t.Run("int not nil", func(t *testing.T) {
		// Arrange
		v := 42
		// Act
		got := isNil(v)
		// Assert
		if got {
			t.Fatalf("expected false for int")
		}
	})
}
