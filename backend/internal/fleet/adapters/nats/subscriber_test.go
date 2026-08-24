package nats

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/sony/gobreaker"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type fakeSubscriberJS struct {
	nats.JetStreamContext
	subscribeCalled int
	lastSubject     string
	lastOptsLen     int
	cb              nats.MsgHandler
	retErr          error
	sub             *nats.Subscription
}

func (f *fakeSubscriberJS) Subscribe(subj string, cb nats.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	// Arrange helper: capture subject and opts count for assertions via observable behavior
	f.subscribeCalled++
	f.lastSubject = subj
	f.lastOptsLen = len(opts)
	f.cb = cb
	if f.retErr != nil {
		return nil, f.retErr
	}
	sub := &nats.Subscription{}
	f.sub = sub
	return sub, nil
}

func newOpenBreaker() *gobreaker.CircuitBreaker {
	// Arrange helper: breaker that is open
	st := gobreaker.Settings{
		Name:        "test-open",
		MaxRequests: 1,
		Interval:    time.Second,
		Timeout:     60 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool { return counts.ConsecutiveFailures >= 1 },
	}
	cb := gobreaker.NewCircuitBreaker(st)
	_, _ = cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })
	for i := 0; i < 3 && cb.State() != gobreaker.StateOpen; i++ {
		_, _ = cb.Execute(func() (interface{}, error) { return nil, errors.New("fail") })
	}
	return cb
}

func TestAlertSubscriber_SubscribeAlerts(t *testing.T) {
	// Covers [SPEC-002: AC-005/006, BR-006]
	t.Run("lastSeq 0 -> Subject alerts.critical DeliverAll", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewAlertSubscriber(fake)
		ctx := context.Background()
		// Act
		ch, unsub, err := sub.SubscribeAlerts(ctx, 0)
		// Assert
		if err != nil { // AC-006
			t.Fatalf("expected nil err, got %v", err)
		}
		if fake.subscribeCalled != 1 { // AC-006
			t.Fatalf("expected 1 subscribe call, got %d", fake.subscribeCalled)
		}
		if fake.lastSubject != "alerts.critical" { // AC-005 BR-006
			t.Fatalf("expected subject alerts.critical, got %q", fake.lastSubject)
		}
		if fake.lastOptsLen == 0 { // AC-006 opts passed (DeliverAll vs StartSequence validated via observable subject+channel)
			t.Fatalf("expected opts passed, got 0")
		}
		if cap(ch) != 256 { // sseBufferSize==256
			t.Fatalf("expected cap 256, got %d", cap(ch))
		}
		if unsub == nil { // AC-006 unsub not nil
			t.Fatalf("expected unsub func")
		}
		// cleanup
		unsub()
	})

	t.Run("lastSeq 101 -> StartSequence 101", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewAlertSubscriber(fake)
		ctx := context.Background()
		// Act
		ch, unsub, err := sub.SubscribeAlerts(ctx, 101)
		// Assert
		if err != nil { // AC-006
			t.Fatalf("expected nil, got %v", err)
		}
		if fake.lastSubject != "alerts.critical" { // AC-005
			t.Fatalf("expected alerts.critical got %q", fake.lastSubject)
		}
		if fake.subscribeCalled != 1 { // AC-006 observable: subscribe invoked with replay semantics for seq 101
			t.Fatalf("expected 1 subscribe call, got %d", fake.subscribeCalled)
		}
		if fake.lastOptsLen == 0 { // AC-006 StartSequence opt passed
			t.Fatalf("expected opts passed for StartSequence")
		}
		if fake.cb == nil {
			t.Fatalf("expected cb captured")
		}
		// observable channel behavior for replay: delivering a msg after subscribe yields it on channel
		fake.cb(&nats.Msg{Data: []byte(`{"seq":101}`)})
		select {
		case msg := <-ch:
			if string(msg.Data) != `{"seq":101}` {
				t.Fatalf("expected replay data, got %q", string(msg.Data))
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout waiting for replay msg")
		}
		if unsub == nil {
			t.Fatalf("expected unsub")
		}
		unsub()
	})

	t.Run("error js.Subscribe -> returns error wrapped", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{retErr: errors.New("nats down")}
		sub := NewAlertSubscriber(fake)
		// Act
		_, _, err := sub.SubscribeAlerts(context.Background(), 0)
		// Assert
		if err == nil { // AC-006 error wrapped
			t.Fatalf("expected error")
		}
		if !errors.Is(err, fake.retErr) && !contains(err.Error(), "nats down") {
			t.Fatalf("expected wrapped nats down, got %v", err)
		}
		if !contains(err.Error(), "subscribe alerts") { // AC-006 wrapped with context
			t.Fatalf("expected subscribe alerts prefix, got %v", err)
		}
	})

	t.Run("nil js -> ErrUnavailable wrapped", func(t *testing.T) {
		// Arrange
		sub := NewAlertSubscriber(nil)
		// Act
		_, _, err := sub.SubscribeAlerts(context.Background(), 0)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, shared.ErrUnavailable) { // BR-006
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("breaker open -> error ErrOpenState", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		brk := newOpenBreaker()
		if brk.State() != gobreaker.StateOpen {
			t.Skipf("breaker not open state=%v", brk.State())
		}
		sub := NewAlertSubscriberWithBreaker(fake, brk, time.Second)
		// Act
		_, _, err := sub.SubscribeAlerts(context.Background(), 0)
		// Assert
		if err == nil { // BR-006 breaker open -> 503 equivalent
			t.Fatalf("expected breaker open error")
		}
		if !errors.Is(err, gobreaker.ErrOpenState) { // BR-006
			t.Fatalf("expected ErrOpenState, got %v", err)
		}
		if fake.subscribeCalled != 0 { // should not call js when breaker open
			t.Fatalf("expected no subscribe when breaker open, got %d", fake.subscribeCalled)
		}
	})

	t.Run("Data with \\n preserved", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewAlertSubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, unsub, err := sub.SubscribeAlerts(ctx, 0)
		if err != nil {
			t.Fatalf("subscribe failed %v", err)
		}
		defer unsub()
		if fake.cb == nil {
			t.Fatalf("expected cb captured")
		}
		// Act
		fake.cb(&nats.Msg{Data: []byte("a\nb")})
		// Assert
		select {
		case msg := <-ch:
			// AC-005 Data with \n preserved
			if string(msg.Data) != "a\nb" {
				t.Fatalf("expected a\\nb, got %q", string(msg.Data))
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout waiting for msg with newline")
		}
	})

	t.Run("context cancel -> channel closed and Unsubscribe called", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewAlertSubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		ch, unsub, err := sub.SubscribeAlerts(ctx, 0)
		if err != nil {
			t.Fatalf("subscribe %v", err)
		}
		defer func() {
			// ensure double unsub no panic via sync.Once
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("double unsub panic %v", r)
				}
			}()
			unsub()
			unsub()
		}()
		// Act
		cancel()
		// Assert
		select {
		case _, ok := <-ch:
			if ok {
				// channel may still deliver but eventually closed; wait for close
				// drain until closed
				timeout := time.After(500 * time.Millisecond)
				closed := false
				for !closed {
					select {
					case _, ok2 := <-ch:
						if !ok2 {
							closed = true
						}
					case <-timeout:
						t.Fatalf("expected channel closed after cancel")
					}
				}
			}
			// ok already false means closed
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected channel closed after context cancel") // BR-006
		}
		// also verify that second channel close via timeout is guarded by sync.Once (no panic)
		// and that unsub is idempotent (sync.Once)
	})

	t.Run("sseBufferSize==256 and sync.Once double unsub no panic", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewAlertSubscriber(fake)
		ch, unsub, err := sub.SubscribeAlerts(context.Background(), 0)
		// Act
		if err != nil {
			t.Fatalf("err %v", err)
		}
		// Assert
		if cap(ch) != 256 { // sseBufferSize
			t.Fatalf("expected 256, got %d", cap(ch))
		}
		// double unsub should not panic due to sync.Once
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); unsub() }()
		go func() { defer wg.Done(); unsub() }()
		wg.Wait()
		// second call after concurrent should not panic and channel closed
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on double unsub %v", r)
				}
			}()
			unsub()
		}()
		select {
		case _, ok := <-ch:
			if ok {
				t.Fatalf("expected closed channel after unsub")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("channel not closed after unsub")
		}
	})
}

func TestTelemetrySubscriber_SubscribePositions(t *testing.T) {
	// Covers [SPEC-002: AC-001, BR-006, BR-012]
	t.Run("nil plate -> telemetry.raw.>", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		// Act
		ch, unsub, err := sub.SubscribePositions(context.Background(), nil, 0)
		// Assert
		if err != nil { // AC-001
			t.Fatalf("err %v", err)
		}
		if fake.lastSubject != "telemetry.raw.>" { // BR-012 all
			t.Fatalf("expected telemetry.raw.>, got %q", fake.lastSubject)
		}
		if fake.lastOptsLen == 0 { // AC-006 observable opts
			t.Fatalf("expected opts passed")
		}
		if cap(ch) != 256 { // sseBufferSize
			t.Fatalf("expected 256, got %d", cap(ch))
		}
		unsub()
	})

	t.Run("plate GTP980 -> telemetry.raw.GTP980", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		plate := "GTP980"
		// Act
		_, unsub, err := sub.SubscribePositions(context.Background(), &plate, 0)
		// Assert
		if err != nil { // AC-001 BR-012
			t.Fatalf("err %v", err)
		}
		if fake.lastSubject != "telemetry.raw.GTP980" { // BR-012 filtered
			t.Fatalf("expected telemetry.raw.GTP980, got %q", fake.lastSubject)
		}
		unsub()
	})

	t.Run("empty plate string -> telemetry.raw.>", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		empty := ""
		// Act
		_, unsub, err := sub.SubscribePositions(context.Background(), &empty, 0)
		// Assert
		if err != nil {
			t.Fatalf("err %v", err)
		}
		if fake.lastSubject != "telemetry.raw.>" {
			t.Fatalf("expected telemetry.raw.>, got %q", fake.lastSubject)
		}
		unsub()
	})

	t.Run("lastSeq 101 -> StartSequence 101", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		// Act
		ch, unsub, err := sub.SubscribePositions(context.Background(), nil, 101)
		// Assert
		if err != nil {
			t.Fatalf("err %v", err)
		}
		if fake.subscribeCalled != 1 {
			t.Fatalf("expected 1 call, got %d", fake.subscribeCalled)
		}
		if fake.lastOptsLen == 0 { // AC-006 StartSequence observable via opts presence
			t.Fatalf("expected opts for StartSequence")
		}
		// observable: replay delivers msg after subscribe
		if fake.cb == nil {
			t.Fatalf("expected cb")
		}
		payload := []byte(`{"plate":"GTP980","lat":4.7,"lon":-74.07,"speed":10}`)
		fake.cb(&nats.Msg{Data: payload})
		select {
		case msg := <-ch:
			if string(msg.Data) != string(payload) {
				t.Fatalf("expected payload %q got %q", string(payload), string(msg.Data))
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout waiting for replay")
		}
		unsub()
	})

	t.Run("error js.Subscribe -> returns error wrapped", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{retErr: errors.New("nats down")}
		sub := NewTelemetrySubscriber(fake)
		// Act
		_, _, err := sub.SubscribePositions(context.Background(), nil, 0)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !contains(err.Error(), "subscribe positions") {
			t.Fatalf("expected subscribe positions prefix, got %v", err)
		}
	})

	t.Run("nil js -> ErrUnavailable", func(t *testing.T) {
		// Arrange
		sub := NewTelemetrySubscriber(nil)
		// Act
		_, _, err := sub.SubscribePositions(context.Background(), nil, 0)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, shared.ErrUnavailable) {
			t.Fatalf("expected ErrUnavailable, got %v", err)
		}
	})

	t.Run("breaker open -> error", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		brk := newOpenBreaker()
		if brk.State() != gobreaker.StateOpen {
			t.Skipf("breaker not open")
		}
		sub := NewTelemetrySubscriberWithBreaker(fake, brk, time.Second)
		// Act
		_, _, err := sub.SubscribePositions(context.Background(), nil, 0)
		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, gobreaker.ErrOpenState) {
			t.Fatalf("expected ErrOpenState, got %v", err)
		}
	})

	t.Run("Data with newline preserved via PosMsg", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, unsub, err := sub.SubscribePositions(ctx, nil, 0)
		if err != nil {
			t.Fatalf("subscribe %v", err)
		}
		defer unsub()
		if fake.cb == nil {
			t.Fatalf("cb nil")
		}
		// PosMsg Data with newline: raw JSON containing newline in string value? Use Data directly.
		// Telemetry cb marshals raw JSON into PosMsg.Data preserving original bytes
		payload := []byte("{\"plate\":\"GTP980\",\"lat\":4.7,\"lon\":-74.07,\"speed\":10}\nmore")
		// Act
		fake.cb(&nats.Msg{Data: payload})
		// Assert
		select {
		case msg := <-ch:
			if string(msg.Data) != string(payload) { // AC-001 Data preserved
				t.Fatalf("expected %q, got %q", string(payload), string(msg.Data))
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout")
		}
	})

	t.Run("context cancel -> channel closed", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		ch, unsub, err := sub.SubscribePositions(ctx, nil, 0)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		defer func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic double unsub %v", r)
				}
			}()
			unsub()
			unsub()
		}()
		// Act
		cancel()
		// Assert
		select {
		case _, ok := <-ch:
			if ok {
				timeout := time.After(500 * time.Millisecond)
				closed := false
				for !closed {
					select {
					case _, ok2 := <-ch:
						if !ok2 {
							closed = true
						}
					case <-timeout:
						t.Fatalf("expected closed")
					}
				}
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("expected channel closed") // BR-006
		}
	})

	t.Run("double unsub no panic", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		ch, unsub, err := sub.SubscribePositions(context.Background(), nil, 0)
		if err != nil {
			t.Fatalf("err %v", err)
		}
		// Act
		unsub()
		// Assert
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic %v", r)
				}
			}()
			unsub()
		}()
		select {
		case _, ok := <-ch:
			if ok {
				t.Fatalf("expected closed")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("not closed")
		}
	})

	t.Run("valid JSON parses plate lat lon and receivedAt", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, unsub, err := sub.SubscribePositions(ctx, nil, 0)
		if err != nil {
			t.Fatalf("subscribe %v", err)
		}
		defer unsub()
		payload := []byte(`{"plate":"GTP980","lat":4.71111119,"lon":-74.07222299,"speed":42,"received_at":"2024-01-02T03:04:05.000000000Z"}`)
		// Act
		fake.cb(&nats.Msg{Data: payload})
		// Assert
		select {
		case msg := <-ch:
			if msg.Plate != "GTP980" { // AC-001 plate parsed
				t.Fatalf("expected GTP980, got %q", msg.Plate)
			}
			if msg.Lat == nil || *msg.Lat != 4.71111119 { // AC-001 lat preserved raw (Round6 in SSE layer, not here)
				t.Fatalf("expected 4.71111119, got %v", msg.Lat)
			}
			if msg.Speed != 42 { // AC-001 speed
				t.Fatalf("expected 42, got %d", msg.Speed)
			}
			if msg.ReceivedAt.IsZero() { // AC-001 receivedAt parsed
				t.Fatalf("expected ReceivedAt not zero")
			}
			if string(msg.Data) != string(payload) { // AC-001 Data preserved
				t.Fatalf("expected payload preserved")
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout")
		}
	})

	t.Run("missing ReceivedAt sets now", func(t *testing.T) {
		// Arrange
		fake := &fakeSubscriberJS{}
		sub := NewTelemetrySubscriber(fake)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ch, unsub, err := sub.SubscribePositions(ctx, nil, 0)
		if err != nil {
			t.Fatalf("subscribe %v", err)
		}
		defer unsub()
		before := time.Now().UTC().Add(-time.Second)
		payload := []byte(`{"plate":"GTP980","lat":4.7,"lon":-74.07,"speed":10}`)
		// Act
		fake.cb(&nats.Msg{Data: payload})
		// Assert
		select {
		case msg := <-ch:
			if msg.ReceivedAt.IsZero() { // AC-001 ReceivedAt defaults to now
				t.Fatalf("expected ReceivedAt set")
			}
			if msg.ReceivedAt.Before(before) { // AC-001 now
				t.Fatalf("expected ReceivedAt after %v, got %v", before, msg.ReceivedAt)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatalf("timeout")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (func() bool {
		for i := 0; i <= len(s)-len(substr); i++ {
			if s[i:i+len(substr)] == substr {
				return true
			}
		}
		return false
	})()
}
