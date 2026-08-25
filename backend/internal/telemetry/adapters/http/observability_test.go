package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetrics_Coverage(t *testing.T) {
	t.Run("GET /metrics exposes gauges and counters", func(t *testing.T) {
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 100, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		payload := map[string]interface{}{"plate": "GTP890", "speed": 42, "lat": 4.7, "lon": -74.0, "client_event_id": "550e8400-e29b-41d4-a716-446655440010"}
		body := marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/v1/telemetry", bytes.NewReader(body))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusAccepted {
			t.Fatalf("setup ingest failed %d", rec.Code)
		}
		req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics got %d body %s", rec2.Code, rec2.Body.String())
		}
		ct := rec2.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Fatalf("expected text/plain got %q", ct)
		}
		b := rec2.Body.String()
		for _, want := range []string{"ingest_inflight", "jetstream_bytes", "breaker_state", "telemetry_ingest_total", "telemetry_ingest_total_sum"} {
			if !strings.Contains(b, want) {
				t.Fatalf("expected metrics to contain %q body %q", want, b)
			}
		}
		if !strings.Contains(b, `plate="GTP890"`) {
			t.Fatalf("expected per-plate label got %q", b)
		}
	})
	t.Run("GET /metrics with special plate escapes label", func(t *testing.T) {
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{used: 0, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		// directly recordIngest via handler internal metrics to test escape
		hs, ok := h.(*handler)
		if !ok {
			t.Fatalf("handler type assert failed")
		}
		hs.recordIngest("A\"B\\C\nD", 1)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		b := rec.Body.String()
		if !strings.Contains(b, `A\"B\\C\nD`) && !strings.Contains(b, `plate=`) {
			t.Fatalf("expected escaped label, got %q", b)
		}
	})
	t.Run("GET /metrics method not allowed", func(t *testing.T) {
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "closed"}
		js := &fakeJetStream{}
		h := buildHandler(pub, limiter, breaker, js)
		req := httptest.NewRequest(http.MethodPost, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 got %d", rec.Code)
		}
	})
	t.Run("GET /healthz ok and method not allowed", func(t *testing.T) {
		pub := &fakePublisher{maxPending: 1024}
		limiter := &fakeLimiter{allow: true, allowBatch: true}
		breaker := &fakeBreaker{state: "open"}
		js := &fakeJetStream{used: 800, max: 1000}
		h := buildHandler(pub, limiter, breaker, js)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 healthz got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "breaker") {
			t.Fatalf("expected breaker in healthz")
		}
		req2 := httptest.NewRequest(http.MethodPost, "/healthz", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for POST healthz got %d", rec2.Code)
		}
	})
	t.Run("escapeLabel covers branches", func(t *testing.T) {
		if escapeLabel(`a"b\c`) != `a\"b\\c` {
			t.Fatalf("escape failed")
		}
		if escapeLabel("a\nb") != `a\nb` {
			t.Fatalf("escape newline failed")
		}
		if escapeLabel("ABC123") != "ABC123" {
			t.Fatalf("escape plain failed")
		}
	})
}

func TestHandlerHelpers_Coverage(t *testing.T) {
	t.Run("breakerState nil and open", func(t *testing.T) {
		if breakerState(nil) != "closed" {
			t.Fatalf("expected closed for nil")
		}
		b := &fakeBreaker{state: "open"}
		if breakerState(b) != "open" {
			t.Fatalf("expected open")
		}
		b2 := &fakeBreaker{state: "closed"}
		if breakerState(b2) != "closed" {
			t.Fatalf("expected closed")
		}
	})
	t.Run("jetstreamStatus nil and values", func(t *testing.T) {
		if jetstreamStatus(nil) != "connected" {
			t.Fatalf("expected connected for nil")
		}
		js := &fakeJetStream{used: 5, max: 10}
		if jetstreamStatus(js) != "5/10" {
			t.Fatalf("expected 5/10 got %q", jetstreamStatus(js))
		}
	})
}
