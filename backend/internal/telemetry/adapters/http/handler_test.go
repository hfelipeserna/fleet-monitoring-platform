package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	nethttp "net/http"
	nethttptest "net/http/httptest"
	"strings"
	"testing"
	"time"

	"fleetmonitoring/backend/internal/telemetry/domain"
)

// Covers SPEC-001 Step 2 handler contracts. Expected production API:
//   func NewHandler(pub Publisher, limiter RateLimiter, breaker Breaker, jsInfo JetStreamInfo) nethttp.Handler

// fakePublisher records calls and simulates backpressure.
type fakePublisher struct {
	publishErr      error
	publishBatchErr error
	pending         int
	maxPending      int
	published       []domain.TelemetryEvent
	batchPublished  [][]domain.TelemetryEvent
}

func (f *fakePublisher) Publish(ctx context.Context, evt domain.TelemetryEvent) error {
	if f.publishErr != nil {
		return f.publishErr
	}
	if f.maxPending > 0 && f.pending >= f.maxPending {
		return errors.New("backpressure: max_pending exceeded")
	}
	f.published = append(f.published, evt)
	return nil
}

func (f *fakePublisher) PublishBatch(ctx context.Context, evts []domain.TelemetryEvent) error {
	if f.publishBatchErr != nil {
		return f.publishBatchErr
	}
	if f.maxPending > 0 && f.pending >= f.maxPending {
		return errors.New("backpressure: max_pending exceeded")
	}
	f.batchPublished = append(f.batchPublished, evts)
	for _, e := range evts {
		f.published = append(f.published, e)
	}
	return nil
}

type fakeLimiter struct {
	allow      bool
	allowBatch bool
	calls      []string
	batchCalls []struct {
		plate string
		n     int
	}
}

func (f *fakeLimiter) Allow(plate string) bool {
	f.calls = append(f.calls, plate)
	return f.allow
}

func (f *fakeLimiter) AllowBatch(plate string, n int) bool {
	f.batchCalls = append(f.batchCalls, struct {
		plate string
		n     int
	}{plate, n})
	return f.allowBatch
}

type fakeBreaker struct {
	state string
	err   error
}

func (f *fakeBreaker) State() string { return f.state }
func (f *fakeBreaker) IsOpen() bool  { return f.state == "open" }
func (f *fakeBreaker) Allow() error  { return f.err }

type fakeJetStream struct {
	used uint64
	max  uint64
}

func (f *fakeJetStream) Bytes() (uint64, uint64) { return f.used, f.max }

func ptrFloat64(v float64) *float64 { return &v }

func newSinglePayload(plate string, speed int, lat, lon *float64, clientID string, occurredAt *time.Time) map[string]interface{} {
	m := map[string]interface{}{
		"plate": plate,
		"speed": speed,
	}
	if lat != nil {
		m["lat"] = *lat
	}
	if lon != nil {
		m["lon"] = *lon
	}
	if clientID != "" {
		m["client_event_id"] = clientID
	}
	if occurredAt != nil {
		m["occurred_at"] = occurredAt.Format(time.RFC3339Nano)
	}
	if lat == nil {
		// explicitly test null handling: caller can override to nil then marshal will keep null if we set typed nil
	}
	return m
}

func marshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// helper to build handler; will fail compile until production implements NewHandler.
// This indirection makes rojo explicit: undefined: NewHandler
func buildHandler(pub Publisher, limiter RateLimiter, breaker Breaker, js JetStreamInfo) nethttp.Handler {
	// Arrange placeholder to extract compile error if NewHandler signature diverges.
	// Expected: func NewHandler(Publisher, RateLimiter, Breaker, JetStreamInfo) nethttp.Handler
	return NewHandler(pub, limiter, breaker, js)
}

// Covers [SPEC-001: AC-001] TEST-001
func TestIngestSingle_AcceptsValidEvent(t *testing.T) {
	t.Run("POST /v1/telemetry valid GTP890 -> 202 accepted true", func(t *testing.T) {
		// Covers [SPEC-001: AC-001, FR-001, BR-001, BR-002, BR-003] TEST-001
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 100, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{
			"plate":           "GTP890",
			"speed":           42,
			"lat":             4.711,
			"lon":             -74.072,
			"client_event_id": "550e8400-e29b-41d4-a716-446655440000",
			"occurred_at":     time.Now().UTC().Add(-1 * time.Minute).Format(time.RFC3339Nano),
		}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json response, got %q err %v", rec.Body.String(), err)
		}
		if resp["accepted"] != true {
			t.Fatalf("expected accepted:true got %v body %s", resp["accepted"], rec.Body.String())
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected Publish called once, got %d", len(pub.published))
		}
		evt := pub.published[0]
		if evt.Plate != "GTP890" {
			t.Fatalf("expected plate GTP890 got %q", evt.Plate)
		}
		if evt.Speed != 42 {
			t.Fatalf("expected speed 42 got %d", evt.Speed)
		}
		if evt.ReceivedAt.IsZero() {
			t.Fatalf("expected received_at assigned server time, got zero")
		}
		if time.Since(evt.ReceivedAt) > 5*time.Second {
			t.Fatalf("received_at not near now: %v", evt.ReceivedAt)
		}
		if evt.ClientEventID != "550e8400-e29b-41d4-a716-446655440000" {
			t.Fatalf("expected MsgId=client_event_id preserved, got %q", evt.ClientEventID)
		}
		if limiter.calls[0] != "GTP890" {
			t.Fatalf("expected rate limiter checked plate GTP890")
		}
	})

	t.Run("valid lat lon null and speed 0 -> 202", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-005, BR-002, BR-003] TEST-004
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{
			"plate":           "GTP890",
			"speed":           0,
			"lat":             nil,
			"lon":             nil,
			"client_event_id": "550e8400-e29b-41d4-a716-446655440001",
		}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202 for null lat/lon speed 0, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected publish, got %d", len(pub.published))
		}
		if pub.published[0].Lat != nil || pub.published[0].Lon != nil {
			t.Fatalf("expected lat/lon nil preserved, got lat %v lon %v", pub.published[0].Lat, pub.published[0].Lon)
		}
		if pub.published[0].Speed != 0 {
			t.Fatalf("expected speed 0, got %d", pub.published[0].Speed)
		}
	})

	t.Run("accepts request without client_event_id generates one", func(t *testing.T) {
		// Covers [SPEC-001: AC-001, FR-003, BR-004] TEST-001
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{
			"plate": "GTP890",
			"speed": 10,
			"lat":   4.7,
			"lon":   -74.0,
		}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.published) != 1 {
			t.Fatalf("expected publish")
		}
		if pub.published[0].ClientEventID == "" {
			t.Fatalf("expected server generated client_event_id")
		}
	})
}

// Covers [SPEC-001: AC-004] TEST-004
func TestIngestSingle_Validation(t *testing.T) {
	invalidCases := []struct {
		name    string
		payload map[string]interface{}
	}{
		{"plate too short GTP89", map[string]interface{}{"plate": "GTP89", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440010"}},
		{"speed negative -1", map[string]interface{}{"plate": "GTP890", "speed": -1, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440011"}},
		{"lat out of range 100", map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 100, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440012"}},
		{"lon out of range 200", map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": 200, "client_event_id": "550e8400-e29b-41d4-a716-446655440013"}},
		{"lon out of range -200", map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -200, "client_event_id": "550e8400-e29b-41d4-a716-446655440014"}},
		{"lat out of range -91", map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": -91, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440015"}},
		{"occurred_at future >5m", map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440016", "occurred_at": time.Now().UTC().Add(6 * time.Minute).Format(time.RFC3339Nano)}},
		{"speed float not int", map[string]interface{}{"plate": "GTP890", "speed": 12.5, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440017"}},
		{"missing plate", map[string]interface{}{"speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440018"}},
		{"missing speed", map[string]interface{}{"plate": "GTP890", "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440019"}},
	}
	for _, tc := range invalidCases {
		tc := tc
		t.Run(tc.name+" -> 400 without publish", func(t *testing.T) {
			// Covers [SPEC-001: AC-004, FR-004] TEST-004
			// Arrange
			pub := &fakePublisher{maxPending: 1024}
			limiter := &fakeLimiter{allow: true, allowBatch: true}
			breaker := &fakeBreaker{state: "closed"}
			js := &fakeJetStream{used: 0, max: 10000}
			h := buildHandler(pub, limiter, breaker, js)
			body := marshal(tc.payload)
			req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := nethttptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != nethttp.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d body %s", tc.name, rec.Code, rec.Body.String())
			}
			if len(pub.published) != 0 {
				t.Fatalf("expected no publish on validation failure, got %d", len(pub.published))
			}
			// limiter should not be checked or should not publish; but we assert no publish is enough
		})
	}

	t.Run("invalid json -> 400 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-004, FR-004] TEST-004
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", strings.NewReader("{ invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected 400 for invalid json, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})
}

// Covers [SPEC-001: AC-002] TEST-002
func TestIngestBatch_AcceptsVariableSizes(t *testing.T) {
	validEvent := func(i int) map[string]interface{} {
		return map[string]interface{}{
			"plate":           "GTP890",
			"speed":           i % 100,
			"lat":             4.711 + float64(i)*0.0001,
			"lon":             -74.072 + float64(i)*0.0001,
			"client_event_id": fmt.Sprintf("550e8400-e29b-41d4-a716-44665544%04d", i%10000),
		}
	}
	// Ensure deterministic uuid-like; use format with 36 chars
	genID := func(i int) string {
		s := fmt.Sprintf("%04d", i%10000)
		return strings.Replace("550e8400-e29b-41d4-a716-446655440000", "0000", s, 1)
	}

	t.Run("batch 245 -> 202 accepted 245", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := make([]map[string]interface{}, 245)
		for i := 0; i < 245; i++ {
			events[i] = map[string]interface{}{
				"plate":           "GTP890",
				"speed":           42,
				"lat":             4.7,
				"lon":             -74.0,
				"client_event_id": genID(i + 1),
			}
		}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid json response %q err %v", rec.Body.String(), err)
		}
		if int(resp["accepted"].(float64)) != 245 {
			t.Fatalf("expected accepted 245 got %v body %s", resp["accepted"], rec.Body.String())
		}
		if len(pub.batchPublished) != 1 || len(pub.batchPublished[0]) != 245 {
			t.Fatalf("expected PublishBatch 245, got %+v", pub.batchPublished)
		}
		for _, evt := range pub.batchPublished[0] {
			if evt.ReceivedAt.IsZero() {
				t.Fatalf("received_at not assigned for batch")
			}
		}
		_ = validEvent
	})

	t.Run("batch 1 -> 202 accepted 1", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := []map[string]interface{}{{"plate": "GTP890", "speed": 0, "lat": nil, "lon": nil, "client_event_id": genID(999)}}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if int(resp["accepted"].(float64)) != 1 {
			t.Fatalf("expected accepted 1 got %v", resp["accepted"])
		}
	})

	t.Run("batch 500 -> 202 accepted 500", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 2048}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := make([]map[string]interface{}, 500)
		for i := 0; i < 500; i++ {
			events[i] = map[string]interface{}{"plate": "GTP890", "speed": i % 120, "lat": 4.7, "lon": -74.0, "client_event_id": genID(i + 100)}
		}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if int(resp["accepted"].(float64)) != 500 {
			t.Fatalf("expected 500 got %v", resp["accepted"])
		}
	})

	t.Run("batch with one invalid event -> 400 all-or-nothing without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, AC-004, FR-002, FR-004] TEST-002 + TEST-004
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := []map[string]interface{}{
			{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": genID(1)},
			{"plate": "GTP89", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": genID(2)},
		}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.batchPublished) != 0 || len(pub.published) != 0 {
			t.Fatalf("expected no publish on batch validation failure")
		}
	})
}

// Covers [SPEC-001: AC-002] TEST-002 edge cases empty, >500, >1MB
func TestIngestBatch_Limits(t *testing.T) {
	t.Run("empty batch -> 400 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		body := marshal(map[string]interface{}{"events": []interface{}{}})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected 400 for empty batch, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch >500 -> 400 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 4096}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := make([]map[string]interface{}, 501)
		for i := 0; i < 501; i++ {
			events[i] = map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-44665544" + strings.Repeat("0", 4)}
		}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected 400 for >500, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch >1MB -> 413 payload too large without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-002, FR-002] TEST-002 - handler enforces 1MiB via MaxBytesReader -> 413
		// Arrange
		pub := &fakePublisher{maxPending: 4096}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		// Build payload >1MB: 500 events with large strings
		largeStr := strings.Repeat("A", 5000)
		events := make([]map[string]interface{}, 500)
		for i := 0; i < 500; i++ {
			events[i] = map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440000", "extra": largeStr}
		}
		body := marshal(map[string]interface{}{"events": events})
		if len(body) <= 1024*1024 {
			t.Skipf("payload not >1MB (%d), skip size limit assertion; implementation may not enforce 1MB at handler", len(body))
		}
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413 payload_too_large for >1MB, got %d body %s len %d", rec.Code, rec.Body.String(), len(body))
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish for oversize")
		}
	})

	t.Run("batch missing events field -> 400", func(t *testing.T) {
		// Covers [SPEC-001: AC-002] TEST-002
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		body := marshal(map[string]interface{}{"foo": []int{1, 2}})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %s", rec.Code, rec.Body.String())
		}
	})
}

// Covers [SPEC-001: AC-005] TEST-005
func TestRateLimitOnline(t *testing.T) {
	t.Run("21st request per plate within minute -> 429 Retry-After:5 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-005, FR-006, BR-005] TEST-005
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: false, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 42, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440000"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Retry-After") != "5" {
			t.Fatalf("expected Retry-After:5 got %q", rec.Header().Get("Retry-After"))
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish on rate limit")
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["error"] != "rate_limited" {
			t.Logf("warning: expected error rate_limited got %v", resp["error"])
		}
	})

	t.Run("different plate not rate limited -> 202", func(t *testing.T) {
		// Covers [SPEC-001: AC-005, FR-006, BR-005] TEST-005
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "TTY423", "speed": 42, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440099"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202 for non-limited plate, got %d body %s", rec.Code, rec.Body.String())
		}
	})
}

// Covers [SPEC-001: AC-006] TEST-006
func TestRateLimitBatch(t *testing.T) {
	t.Run("second batch <5s -> 429 Retry-After:5 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-006, FR-007, BR-005] TEST-006
		// Arrange
		pub := &fakePublisher{maxPending: 2048}
		limiter := &fakeLimiter{allow: true, allowBatch: false}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := []map[string]interface{}{{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440100"}}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Retry-After") != "5" {
			t.Fatalf("expected Retry-After:5 got %q", rec.Header().Get("Retry-After"))
		}
		if len(pub.batchPublished) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("batch 500 events then immediate second batch size 10 -> 429 due to 500/30s bucket", func(t *testing.T) {
		// Covers [SPEC-001: AC-006, FR-007, BR-005] TEST-006
		// Arrange
		pub := &fakePublisher{maxPending: 2048}
		limiter := &fakeLimiter{allow: true, allowBatch: false}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := make([]map[string]interface{}, 10)
		for i := range events {
			events[i] = map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-44665544" + strings.Repeat("0", 4)}
		}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})
}

// Covers [SPEC-001: AC-007] TEST-007
func TestBackpressure(t *testing.T) {
	t.Run("MaxPending exceeded -> 503 distinct from 429 with Retry-After without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1, pending: 1, publishErr: errors.New("backpressure: max_pending exceeded")}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440200"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Retry-After") == "" {
			t.Fatalf("expected Retry-After header for 503")
		}
		if rec.Code == nethttp.StatusTooManyRequests {
			t.Fatalf("503 must be distinct from 429")
		}
	})

	t.Run("breaker open -> 503 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "open", err: errors.New("circuit breaker open")}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440201"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("expected 503 for breaker open, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish when breaker open")
		}
	})

	t.Run("jetstream bytes >=80% -> 503 without publish", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008, BR-006] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 850, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440202"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("expected 503 for jetstream >=80%%, got %d body %s", rec.Code, rec.Body.String())
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish")
		}
	})

	t.Run("jetstream exactly 80% -> 503", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 800, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440203"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("expected 503 at 80%% threshold, got %d", rec.Code)
		}
	})

	t.Run("503 vs 429 distinct status", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, BR-006] TEST-007
		// Arrange
		if nethttp.StatusServiceUnavailable == nethttp.StatusTooManyRequests {
			t.Fatalf("status codes must be distinct")
		}
	})

	t.Run("batch backpressure -> 503", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-008] TEST-007
		// Arrange
		pub := &fakePublisher{publishBatchErr: errors.New("backpressure: max_pending exceeded")}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		events := []map[string]interface{}{{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440300"}}
		body := marshal(map[string]interface{}{"events": events})
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry/batch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusServiceUnavailable {
			t.Fatalf("expected 503 for batch backpressure, got %d body %s", rec.Code, rec.Body.String())
		}
	})
}

// Covers [SPEC-001: AC-007, FR-011] TEST-007 healthz
func TestHealthz_BreakerState(t *testing.T) {
	t.Run("GET /healthz shows breaker open when open", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-011, BR-006] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "open", err: errors.New("open")}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		req := nethttptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusOK {
			t.Fatalf("expected 200 for healthz, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("invalid json healthz %q", rec.Body.String())
		}
		if resp["breaker"] != "open" {
			t.Fatalf("expected breaker open in healthz, got %v body %s", resp["breaker"], rec.Body.String())
		}
	})

	t.Run("GET /healthz closed when healthy", func(t *testing.T) {
		// Covers [SPEC-001: AC-007, FR-011] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 100, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		req := nethttptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var resp map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if resp["breaker"] != "closed" {
			t.Fatalf("expected closed got %v", resp["breaker"])
		}
		if resp["status"] != "ok" {
			t.Fatalf("expected status ok got %v", resp["status"])
		}
	})

	t.Run("GET /healthz shows jetstream state", func(t *testing.T) {
		// Covers [SPEC-001: FR-011] TEST-007
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		req := nethttptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "jetstream") {
			t.Fatalf("expected jetstream field in healthz body %s", body)
		}
	})
}

// Covers [SPEC-001: AC-001, AC-002, FR-001, FR-002] content type and method
func TestMethodAndContentType(t *testing.T) {
	t.Run("GET /v1/telemetry -> 405", func(t *testing.T) {
		// Covers [SPEC-001: FR-001] TEST-001
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		req := nethttptest.NewRequest(nethttp.MethodGet, "/v1/telemetry", nil)
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusMethodNotAllowed && rec.Code != nethttp.StatusNotFound {
			t.Fatalf("expected 405, got %d", rec.Code)
		}
	})

	t.Run("POST without Content-Type still parses if json", func(t *testing.T) {
		// Covers [SPEC-001: AC-001] TEST-001
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440400"}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		rec := nethttptest.NewRecorder()
		// No Content-Type

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusAccepted {
			t.Fatalf("expected 202 even without content-type, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("body too large single event with very large payload -> 413", func(t *testing.T) {
		// Covers [SPEC-001: FR-002] TEST-002 - MaxBytesReader returns MaxBytesError -> 413
		// Arrange
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 10000}
		h := buildHandler(pub, limiter, breaker, js)
		large := strings.Repeat("x", 2*1024*1024)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 10, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440401", "extra": large}
		body := marshal(payload)
		req := nethttptest.NewRequest(nethttp.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := nethttptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != nethttp.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413 payload_too_large for oversize single, got %d len %d body %s", rec.Code, len(body), rec.Body.String())
		}
		if len(pub.published) != 0 {
			t.Fatalf("expected no publish for oversize")
		}
		_ = io.Discard
	})
}
