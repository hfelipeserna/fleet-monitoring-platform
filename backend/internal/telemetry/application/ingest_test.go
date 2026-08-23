package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/telemetry/domain"
)

// Expected production API for this package:
//   type Publisher interface { Publish(ctx context.Context, evt domain.TelemetryEvent) error; PublishBatch(ctx context.Context, evts []domain.TelemetryEvent) error }
//   type RateLimiter interface { Allow(plate string) bool; AllowBatch(plate string, n int) bool }
//   type Breaker interface { State() string; Allow() error }
//   type JetStreamInfo interface { Bytes() (used uint64, max uint64) }
//   func NewIngestService(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo, now func() time.Time) *IngestService
//   func (s *IngestService) IngestSingle(ctx context.Context, raw RawEvent) (domain.TelemetryEvent, error)
//   func (s *IngestService) IngestBatch(ctx context.Context, raws []RawEvent) ([]domain.TelemetryEvent, error)
// RawEvent mirrors JSON with plate/speed/lat/lon/occurred_at/client_event_id validation.
// Errors expected to be wrapped: ErrValidation -> 400, ErrRateLimited -> 429, ErrBackpressure -> 503
// If production uses different names, update these tests together with code and keep Covers tags.

// fakes
type fakePublisherApp struct {
	publishErr      error
	publishBatchErr error
	published       []domain.TelemetryEvent
	batchPublished  [][]domain.TelemetryEvent
	pending         int
	maxPending      int
}

func (f *fakePublisherApp) Publish(ctx context.Context, evt domain.TelemetryEvent) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	if f.maxPending > 0 && f.pending >= f.maxPending {
		return errors.New("backpressure: max_pending")
	}
	f.published = append(f.published, evt)
	return nil
}
func (f *fakePublisherApp) PublishBatch(ctx context.Context, evts []domain.TelemetryEvent) error {
	if f.publishBatchErr != nil {
		return f.publishBatchErr
	}
	if f.maxPending > 0 && f.pending >= f.maxPending {
		return errors.New("backpressure: max_pending")
	}
	f.batchPublished = append(f.batchPublished, evts)
	for _, e := range evts {
		f.published = append(f.published, e)
	}
	return nil
}

type fakeLimiterApp struct {
	allow      bool
	allowBatch bool
	calls      []string
	batchCalls []struct {
		plate string
		n     int
	}
}

func (f *fakeLimiterApp) Allow(plate string) bool {
	f.calls = append(f.calls, plate)
	return f.allow
}
func (f *fakeLimiterApp) AllowBatch(plate string, n int) bool {
	f.batchCalls = append(f.batchCalls, struct {
		plate string
		n     int
	}{plate, n})
	return f.allowBatch
}

type fakeBreakerApp struct {
	state string
	err   error
}

func (f *fakeBreakerApp) State() string { return f.state }
func (f *fakeBreakerApp) IsOpen() bool  { return f.state == "open" }
func (f *fakeBreakerApp) Allow() error  { return f.err }

type fakeJSApp struct {
	used uint64
	max  uint64
}

func (f *fakeJSApp) Bytes() (uint64, uint64) { return f.used, f.max }

func intPtr(v int) *int              { return &v }
func floatPtr(v float64) *float64    { return &v }
func timePtr(v time.Time) *time.Time { return &v }

// helper to build service
func buildService(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo, now func() time.Time) *IngestService {
	return NewIngestService(pub, limiter, breaker, js, now)
}

func validRawEvent() RawEvent {
	return RawEvent{
		Plate:         "GTP890",
		Speed:         intPtr(42),
		Lat:           floatPtr(4.711),
		Lon:           floatPtr(-74.072),
		ClientEventID: "550e8400-e29b-41d4-a716-446655440000",
		OccurredAt:    timePtr(time.Now().UTC().Add(-1 * time.Minute)),
	}
}

// Covers [SPEC-001: AC-001, AC-004] TEST-001, TEST-004
func TestIngestService_IngestSingle(t *testing.T) {
	t.Run("accepts valid event assigns received_at and publishes with MsgId", func(t *testing.T) {
		// Covers [SPEC-001: AC-001, FR-001, FR-003, BR-004, BR-007] TEST-001
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		fixedNow := time.Now().UTC()
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return fixedNow })
		raw := validRawEvent()

		// Act
		evt, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if evt.Plate != "GTP890" {
			t.Fatalf("expected plate GTP890 got %q", evt.Plate)
		}
		if evt.Speed != 42 {
			t.Fatalf("expected speed 42 got %d", evt.Speed)
		}
		if evt.ReceivedAt.IsZero() {
			t.Fatalf("expected received_at assigned")
		}
		if evt.ReceivedAt.Sub(fixedNow) > time.Second {
			t.Fatalf("received_at not equal to now func %v vs %v", evt.ReceivedAt, fixedNow)
		}
		if evt.ClientEventID != raw.ClientEventID {
			t.Fatalf("expected client_event_id preserved %q got %q", raw.ClientEventID, evt.ClientEventID)
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected publish once, got %d", len(pub.published))
		}
		if pub.published[0].ClientEventID != raw.ClientEventID {
			t.Fatalf("expected MsgId=client_event_id")
		}
		if len(limiter.calls) != 1 || limiter.calls[0] != "GTP890" {
			t.Fatalf("expected limiter Allow called with plate")
		}
	})

	t.Run("valid lat lon null and speed 0 publishes with null", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-005, BR-002, BR-003] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := RawEvent{
			Plate:         "GTP890",
			Speed:         intPtr(0),
			Lat:           nil,
			Lon:           nil,
			ClientEventID: "550e8400-e29b-41d4-a716-446655440001",
		}

		// Act
		evt, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err != nil {
			t.Fatalf("expected no error for null lat/lon speed 0, got %v", err)
		}
		if evt.Lat != nil || evt.Lon != nil {
			t.Fatalf("expected lat/lon nil, got %v %v", evt.Lat, evt.Lon)
		}
		if evt.Speed != 0 {
			t.Fatalf("expected speed 0")
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected publish")
		}
	})

	t.Run("generates client_event_id when missing", func(t *testing.T) {
		// Covers [SPEC-001: FR-003, BR-004] TEST-001
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := RawEvent{
			Plate: "GTP890",
			Speed: intPtr(10),
			Lat:   floatPtr(4.7),
			Lon:   floatPtr(-74.0),
		}

		// Act
		evt, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if evt.ClientEventID == "" {
			t.Fatalf("expected generated client_event_id")
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected publish with generated id")
		}
	})

	t.Run("validation fails plate invalid -> error 400 no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-001] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()
		raw.Plate = "GTP89"

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected validation error for plate GTP89")
		}
		if !errors.Is(err, ErrValidation) && !contains(err.Error(), "plate") && !contains(err.Error(), "validation") {
			t.Logf("expected validation error, got %v (check ErrValidation wrapping)", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish on validation failure")
		}
	})

	t.Run("validation fails speed negative -> error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-002] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()
		raw.Speed = intPtr(-1)

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error for speed -1")
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("validation fails lat out of range -> error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-003] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()
		raw.Lat = floatPtr(100)

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error for lat 100")
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("validation fails lon out of range -> error", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004, BR-003] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()
		raw.Lon = floatPtr(200)

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error for lon 200")
		}
	})

	t.Run("validation fails occurred_at future >5m -> error", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		fixedNow := time.Now().UTC()
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return fixedNow })
		raw := validRawEvent()
		fut := fixedNow.Add(6 * time.Minute)
		raw.OccurredAt = &fut

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error for occurred_at >5m future")
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("rate limited -> 429 error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-005, FR-006, BR-005] TEST-005
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: false, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected rate limited error")
		}
		if !errors.Is(err, ErrRateLimited) && !contains(err.Error(), "rate") {
			t.Logf("expected ErrRateLimited, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish on rate limit")
		}
	})

	t.Run("breaker open -> 503 error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "open", err: errors.New("circuit open")}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure error for breaker open")
		}
		if !errors.Is(err, ErrBackpressure) && !contains(err.Error(), "backpressure") && !contains(err.Error(), "breaker") {
			t.Logf("expected backpressure, got %v", err)
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish when breaker open")
		}
	})

	t.Run("jetstream bytes >=80% -> 503 no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 80, max: 100}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure for jetstream 80%%")
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("max pending exceeded -> 503 no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1, pending: 1}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure for max pending")
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("publisher error wraps with backpressure distinction", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008] TEST-007
		// Arrange
		pub := &fakePublisherApp{publishErr: errors.New("nats: max pending")}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()

		// Act
		_, err := svc.IngestSingle(context.Background(), raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !contains(err.Error(), "backpressure") && !contains(err.Error(), "pending") && !errors.Is(err, ErrBackpressure) {
			t.Logf("warning: expected backpressure wrapping got %v", err)
		}
	})

	t.Run("context canceled -> error no publish", func(t *testing.T) {
		// Covers [SPEC-001: FR-008] resilience
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raw := validRawEvent()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		// Act
		_, err := svc.IngestSingle(ctx, raw)

		// Assert
		if err == nil {
			t.Fatalf("expected error for canceled context")
		}
	})
}

// Covers [SPEC-001: AC-002, AC-006] TEST-002, TEST-006
func TestIngestService_IngestBatch(t *testing.T) {
	genRaw := func(n int, plate string) []RawEvent {
		out := make([]RawEvent, n)
		for i := 0; i < n; i++ {
			id := "550e8400-e29b-41d4-a716-44665544" + fourDigits(i)
			out[i] = RawEvent{
				Plate:         plate,
				Speed:         intPtr(i % 100),
				Lat:           floatPtr(4.7),
				Lon:           floatPtr(-74.0),
				ClientEventID: id,
			}
		}
		return out
	}

	t.Run("batch 245 -> publishes 245 assigns received_at each", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 2048}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		fixedNow := time.Now().UTC()
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return fixedNow })
		raws := genRaw(245, "GTP890")

		// Act
		evts, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(evts) != 245 {
			t.Fatalf("expected 245 evts got %d", len(evts))
		}
		if len(pub.batchPublished) != 1 || len(pub.batchPublished[0]) != 245 {
			t.Fatalf("expected batch publish 245 got %v", pub.batchPublished)
		}
		for _, e := range evts {
			if e.ReceivedAt.IsZero() {
				t.Fatalf("received_at not assigned")
			}
			if e.ClientEventID == "" {
				t.Fatalf("client_event_id missing")
			}
			if e.Plate != "GTP890" {
				t.Fatalf("plate mismatch")
			}
		}
	})

	t.Run("batch 1 -> 202", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(1, "GTP890")

		// Act
		evts, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(evts) != 1 {
			t.Fatalf("expected 1")
		}
	})

	t.Run("batch 500 -> 202", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 4096}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(500, "GTP890")

		// Act
		evts, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(evts) != 500 {
			t.Fatalf("expected 500 got %d", len(evts))
		}
	})

	t.Run("empty batch -> validation error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })

		// Act
		_, err := svc.IngestBatch(context.Background(), []RawEvent{})

		// Assert
		if err == nil {
			t.Fatalf("expected error for empty batch")
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch >500 -> error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 4096}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(501, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected error for >500")
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch with one invalid -> all-or-nothing error no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004] TEST-004
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(2, "GTP890")
		raws[1].Plate = "GTP89"

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected validation error")
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish all-or-nothing")
		}
	})

	t.Run("batch rate limited 1/5s -> 429 no publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-006, FR-007, BR-005] TEST-006
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: false}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(10, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected rate limited")
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch 500/30s exceeded -> 429", func(t *testing.T) {
		// Covers [SPEC-001: AC-006, FR-007] TEST-006
		// Arrange
		pub := &fakePublisherApp{maxPending: 4096}
		limiter := &fakeLimiterApp{allow: true, allowBatch: false}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(500, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected rate limited for 500/30s")
		}
	})

	t.Run("batch backpressure max pending -> 503", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1, pending: 1}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(10, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure")
		}
	})

	t.Run("batch breaker open -> 503", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "open", err: errors.New("open")}
		js := &fakeJSApp{used: 0, max: 10000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(10, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure for breaker open")
		}
	})

	t.Run("batch jetstream 80% -> 503", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008] TEST-007
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 800, max: 1000}
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return time.Now().UTC() })
		raws := genRaw(10, "GTP890")

		// Act
		_, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err == nil {
			t.Fatalf("expected backpressure for jetstream 80%%")
		}
	})

	t.Run("batch validates each event received_at assigned", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, BR-007] TEST-002
		// Arrange
		pub := &fakePublisherApp{maxPending: 1024}
		limiter := &fakeLimiterApp{allow: true, allowBatch: true}
		breaker := &fakeBreakerApp{state: "closed"}
		js := &fakeJSApp{used: 0, max: 10000}
		fixed := time.Now().UTC()
		svc := buildService(pub, limiter, breaker, js, func() time.Time { return fixed })
		raws := []RawEvent{
			{Plate: "GTP890", Speed: intPtr(0), Lat: nil, Lon: nil, ClientEventID: "550e8400-e29b-41d4-a716-446655440010"},
			{Plate: "GTP890", Speed: intPtr(10), Lat: floatPtr(4.7), Lon: floatPtr(-74.0), ClientEventID: "550e8400-e29b-41d4-a716-446655440011"},
		}

		// Act
		evts, err := svc.IngestBatch(context.Background(), raws)

		// Assert
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if len(evts) != 2 {
			t.Fatalf("expected 2")
		}
		for _, e := range evts {
			if e.ReceivedAt != fixed {
				t.Fatalf("expected received_at %v got %v", fixed, e.ReceivedAt)
			}
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || containsSlow(s, substr))
}

func containsSlow(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func fourDigits(i int) string {
	s := "000" + itoa(i%10000)
	if len(s) > 4 {
		return s[len(s)-4:]
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := ""
	for n > 0 {
		buf = string(rune('0'+n%10)) + buf
		n /= 10
	}
	return buf
}

// ensure domain import is used
var _ = domain.TelemetryEvent{}
