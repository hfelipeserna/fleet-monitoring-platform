package http

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Covers [SPEC-003: AC-004, AC-008, BR-005, BR-006]

func TestCoverage_OpsHelpers(t *testing.T) {
	t.Run("NewOpsHandlerWithLogger and loggerOrDefault", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		h := NewOpsHandlerWithLogger(prov, logger)
		h2 := NewOpsHandler(nil)

		// Act
		l1 := h.loggerOrDefault()
		l2 := h2.loggerOrDefault()

		// Assert
		if l1 != logger {
			t.Fatalf("expected provided logger")
		}
		if l2 == nil {
			t.Fatalf("expected default logger not nil")
		}
		if buf.Len() != 0 {
			// not used yet
		}
		_ = buf
	})

	t.Run("breakerState nil empty whitespace trimmed", func(t *testing.T) {
		// Arrange
		hNil := &OpsHandler{provider: nil}
		hEmpty := &OpsHandler{provider: &fakeProvForCoverage{breaker: "", db: "ok", gemini: "connected"}}
		hSpace := &OpsHandler{provider: &fakeProvForCoverage{breaker: "  open  ", db: "ok", gemini: "connected"}}
		hValid := &OpsHandler{provider: &fakeProvForCoverage{breaker: "open", db: "ok", gemini: "connected"}}

		// Act
		a := hNil.breakerState()
		b := hEmpty.breakerState()
		c := hSpace.breakerState()
		d := hValid.breakerState()

		// Assert
		if a != "closed" {
			t.Fatalf("expected closed for nil, got %q", a)
		}
		if b != "closed" {
			t.Fatalf("expected closed for empty, got %q", b)
		}
		if c != "open" {
			t.Fatalf("expected open trimmed, got %q", c)
		}
		if d != "open" {
			t.Fatalf("expected open, got %q", d)
		}
	})

	t.Run("dbPool nil empty whitespace and geminiState same", func(t *testing.T) {
		// Arrange
		hNil := &OpsHandler{provider: nil}
		hEmpty := &OpsHandler{provider: &fakeProvForCoverage{db: "", gemini: ""}}
		hSpace := &OpsHandler{provider: &fakeProvForCoverage{db: "  pool error  ", gemini: "  unavailable  "}}
		hOk := &OpsHandler{provider: &fakeProvForCoverage{db: "ok", gemini: "connected"}}

		// Act
		a1 := hNil.dbPool()
		b1 := hEmpty.dbPool()
		c1 := hSpace.dbPool()
		d1 := hOk.dbPool()
		a2 := hNil.geminiState()
		b2 := hEmpty.geminiState()
		c2 := hSpace.geminiState()
		d2 := hOk.geminiState()

		// Assert
		if a1 != "ok" || b1 != "ok" {
			t.Fatalf("expected ok for nil/empty dbPool, got %q %q", a1, b1)
		}
		if c1 != "pool error" {
			t.Fatalf("expected trimmed pool error, got %q", c1)
		}
		if d1 != "ok" {
			t.Fatalf("expected ok, got %q", d1)
		}
		if a2 != "connected" || b2 != "connected" {
			t.Fatalf("expected connected for nil/empty gemini, got %q %q", a2, b2)
		}
		if c2 != "unavailable" {
			t.Fatalf("expected unavailable, got %q", c2)
		}
		if d2 != "connected" {
			t.Fatalf("expected connected, got %q", d2)
		}
	})

	t.Run("withRequestID generates when missing and preserves when present", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		h := NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		// no header
		rec := httptest.NewRecorder()
		// Act
		id := h.withRequestID(rec, req)
		// Assert
		if id == "" {
			t.Fatalf("expected generated request_id")
		}
		if got := rec.Header().Get("X-Request-ID"); got != id {
			t.Fatalf("expected header %q got %q", id, got)
		}
		// with header
		req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req2.Header.Set("X-Request-ID", "  550e8400-e29b-41d4-a716-446655440001  ")
		rec2 := httptest.NewRecorder()
		id2 := h.withRequestID(rec2, req2)
		if id2 != "550e8400-e29b-41d4-a716-446655440001" {
			t.Fatalf("expected trimmed header, got %q", id2)
		}
	})
}

func TestCoverage_OpsServeHTTP(t *testing.T) {
	t.Run("ServeHTTP routes healthz metrics api/metrics chat and notfound", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		h := NewOpsHandler(prov)

		// Act healthz
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 healthz, got %d", rec.Code)
		}
		// Act metrics
		req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics, got %d", rec2.Code)
		}
		if !strings.Contains(rec2.Body.String(), "breaker_state") {
			t.Fatalf("expected breaker_state, got %q", rec2.Body.String())
		}
		// Act api/metrics
		req3 := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)
		if rec3.Code != http.StatusOK {
			t.Fatalf("expected 200 api/metrics, got %d", rec3.Code)
		}
		// Act chat closed -> 200
		req4 := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hi"}`))
		req4.Header.Set("Content-Type", "application/json")
		rec4 := httptest.NewRecorder()
		h.ServeHTTP(rec4, req4)
		if rec4.Code != http.StatusOK {
			t.Fatalf("expected 200 chat closed, got %d body %s", rec4.Code, rec4.Body.String())
		}
		// Act notfound
		req5 := httptest.NewRequest(http.MethodGet, "/unknown", nil)
		rec5 := httptest.NewRecorder()
		h.ServeHTTP(rec5, req5)
		if rec5.Code != http.StatusNotFound {
			t.Fatalf("expected 404 notfound, got %d", rec5.Code)
		}
	})

	t.Run("handleChat breaker open ->503 with Retry-After and metricsPayload open", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "open", db: "ok", gemini: "connected"}
		h := NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hello"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 chat open, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Fatalf("expected Retry-After 30, got %q", got)
		}
		// also test handleMetrics open vs closed
		reqM := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		recM := httptest.NewRecorder()
		h.ServeHTTP(recM, reqM)
		if !strings.Contains(recM.Body.String(), "breaker_state 1") {
			t.Fatalf("expected breaker_state 1 for open, got %q", recM.Body.String())
		}
		prov2 := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		h2 := NewOpsHandler(prov2)
		reqM2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		recM2 := httptest.NewRecorder()
		h2.ServeHTTP(recM2, reqM2)
		if !strings.Contains(recM2.Body.String(), "breaker_state 0") {
			t.Fatalf("expected breaker_state 0 for closed, got %q", recM2.Body.String())
		}
	})

	t.Run("handleHealthz breaker open -> 503 degraded and gemini/db variations", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "OPEN", db: "pool error", gemini: "unavailable"}
		h := NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 open, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "degraded") {
			t.Fatalf("expected degraded, got %q", body)
		}
		if !strings.Contains(body, "pool error") {
			t.Fatalf("expected pool error db, got %q", body)
		}
		if !strings.Contains(body, "unavailable") {
			t.Fatalf("expected unavailable gemini, got %q", body)
		}
		// closed
		prov2 := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		h2 := NewOpsHandler(prov2)
		req2 := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec2 := httptest.NewRecorder()
		h2.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200 closed, got %d", rec2.Code)
		}
	})

	t.Run("metricsPayload includes agent_requests_total and X-Request-ID header", func(t *testing.T) {
		// Arrange
		prov := &fakeProvForCoverage{breaker: "closed", db: "ok", gemini: "connected"}
		h := NewOpsHandler(prov)
		// generate some requests to increment counters
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
		}
		reqM := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		reqM.Header.Set("X-Request-ID", "550e8400-e29b-41d4-a716-446655440099")
		recM := httptest.NewRecorder()
		// Act
		h.ServeHTTP(recM, reqM)
		// Assert
		body := recM.Body.String()
		if !strings.Contains(body, "agent_requests_total") {
			t.Fatalf("expected agent_requests_total, got %q", body)
		}
		if !strings.Contains(body, "agent_tokens_total") {
			t.Fatalf("expected tokens")
		}
		if got := recM.Header().Get("X-Request-ID"); got != "550e8400-e29b-41d4-a716-446655440099" {
			t.Fatalf("expected request-id echo, got %q", got)
		}
		if !strings.Contains(body, "agent_requests_total") {
			t.Fatalf("missing")
		}
	})
}

// fakeProvForCoverage implements HealthProvider for coverage tests
type fakeProvForCoverage struct {
	breaker string
	db      string
	gemini  string
}
func (f *fakeProvForCoverage) BreakerState() string { return f.breaker }
func (f *fakeProvForCoverage) DBPool() string       { return f.db }
func (f *fakeProvForCoverage) GeminiState() string  { return f.gemini }
