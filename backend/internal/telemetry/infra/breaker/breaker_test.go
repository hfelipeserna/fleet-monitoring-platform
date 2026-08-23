package breaker

import (
	"testing"
	"time"
)

func TestBreakerOpensAfterFailures(t *testing.T) {
	b := NewBreaker()
	for i := 0; i < 10; i++ {
		b.RecordFailure()
	}
	if !b.IsOpen() {
		t.Fatalf("expected breaker open after 10 failures, got state %s", b.State())
	}
	if b.State() != "open" {
		t.Fatalf("expected state open got %s", b.State())
	}
	if err := b.Allow(); err == nil {
		t.Fatalf("expected Allow error when open")
	}
}

func TestBreakerNotOpenBeforeThreshold(t *testing.T) {
	b := NewBreaker()
	for i := 0; i < 9; i++ {
		b.RecordFailure()
	}
	if b.IsOpen() {
		t.Fatalf("should not open before 10 requests, state %s", b.State())
	}
	if err := b.Allow(); err != nil {
		t.Fatalf("expected Allow nil when closed, got %v", err)
	}
}

func TestBreakerHalfRatio(t *testing.T) {
	b := NewBreaker()
	for i := 0; i < 5; i++ {
		b.RecordSuccess()
	}
	for i := 0; i < 5; i++ {
		b.RecordFailure()
	}
	if !b.IsOpen() {
		t.Fatalf("expected open at 50%% failure ratio with 10 requests, got %s", b.State())
	}
}

func TestBreakerAllowDoesNotTrip(t *testing.T) {
	b := NewBreaker()
	for i := 0; i < 9; i++ {
		if err := b.Allow(); err != nil {
			t.Fatalf("Allow should not error when closed %v", err)
		}
	}
	if b.IsOpen() {
		t.Fatalf("Allow with nil Execute should not cause trip, state %s", b.State())
	}
}

func TestBreakerWithSettings(t *testing.T) {
	b := NewBreakerWithSettings("test", 10, 30*time.Second, 0.5, 10)
	if b == nil {
		t.Fatalf("expected breaker")
	}
	if b.State() != "closed" {
		t.Fatalf("expected closed, got %s", b.State())
	}
}
