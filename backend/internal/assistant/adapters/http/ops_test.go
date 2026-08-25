package http_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	asshttp "fleetmonitoring/backend/internal/assistant/adapters/http"
)

// Covers [SPEC-003: AC-004, AC-008, BR-005, BR-006]

// fakeOpsProvider mocks OpsProvider{BreackerState,NatsConnected,DBPool,GeminiState}
// Consumer-side interface: defined where used (assistant/adapters/http).
type fakeOpsProvider struct {
	breakerState string
	nats         bool
	dbPool       string
	gemini       string
}

func (f *fakeOpsProvider) BreakerState() string { return f.breakerState }
func (f *fakeOpsProvider) NatsConnected() bool  { return f.nats }
func (f *fakeOpsProvider) DBPool() string       { return f.dbPool }
func (f *fakeOpsProvider) GeminiState() string  { return f.gemini }

// Alternative naming for compatibility with fleet OpsProvider
func (f *fakeOpsProvider) DBPoolStat() string { return f.dbPool }

func TestOpsHandler_Healthz_200(t *testing.T) {
	// Covers [SPEC-003: AC-008, BR-006]
	t.Run("GET /healthz 200 status ok breaker closed gemini connected db ok", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "closed", nats: true, dbPool: "ok", gemini: "connected"}
		h := asshttp.NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 /healthz, got %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected json healthz, got %q err %v", rec.Body.String(), err)
		}
		if body["status"] != "ok" {
			t.Fatalf("expected status ok, got %v body %s", body["status"], rec.Body.String())
		}
		if body["breaker"] != "closed" {
			t.Fatalf("expected breaker closed, got %q body %s", body["breaker"], rec.Body.String())
		}
		if body["gemini"] != "connected" {
			t.Fatalf("expected gemini connected, got %q body %s", body["gemini"], rec.Body.String())
		}
		if body["db"] != "ok" {
			t.Fatalf("expected db ok, got %q body %s", body["db"], rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}
	})
}

func TestOpsHandler_Healthz_BreakerOpen_503(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005]
	t.Run("breaker open -> 503 Retry-After:30 status degraded breaker open", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "open", nats: true, dbPool: "ok", gemini: "unavailable"}
		h := asshttp.NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 /healthz when breaker open, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Fatalf("expected Retry-After:30, got %q body %s", got, rec.Body.String())
		}
		var body map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("expected json body, got %q err %v", rec.Body.String(), err)
		}
		if body["status"] != "degraded" {
			t.Fatalf("expected status degraded, got %q body %s", body["status"], rec.Body.String())
		}
		if body["breaker"] != "open" {
			t.Fatalf("expected breaker open, got %q body %s", body["breaker"], rec.Body.String())
		}
	})
}

func TestOpsHandler_Metrics_exposes_agent(t *testing.T) {
	// Covers [SPEC-003: AC-008, BR-006]
	t.Run("GET /metrics 200 contains agent_requests_total agent_tool_calls_total agent_latency_ms agent_tokens_total breaker_state", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "closed", nats: true, dbPool: "ok", gemini: "connected"}
		h := asshttp.NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 /metrics, got %d body %s", rec.Code, rec.Body.String())
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") {
			t.Fatalf("expected Content-Type text/plain for prometheus, got %q", ct)
		}
		body := rec.Body.String()
		for _, want := range []string{
			"agent_requests_total",
			"agent_tool_calls_total",
			"agent_latency_ms",
			"agent_tokens_total",
			"breaker_state",
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("expected /metrics to contain %q, got %q", want, body)
			}
		}
		if !strings.Contains(body, "# HELP") || !strings.Contains(body, "# TYPE") {
			t.Fatalf("expected prometheus exposition HELP/TYPE, got %q", body)
		}
	})
}

func TestOpsHandler_Fallback_503(t *testing.T) {
	// Covers [SPEC-003: AC-004, BR-005]
	t.Run("chat fallback agente temporalmente no disponible cuando breaker open", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "open", nats: true, dbPool: "ok", gemini: "unavailable"}
		h := asshttp.NewOpsHandler(prov)
		req := httptest.NewRequest(http.MethodPost, "/api/chat", strings.NewReader(`{"message":"hola"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 fallback when breaker open, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "30" {
			t.Fatalf("expected Retry-After:30 on fallback, got %q", got)
		}
		body := strings.ToLower(rec.Body.String())
		if !strings.Contains(body, "agente temporalmente no disponible") {
			t.Fatalf("expected fallback message 'agente temporalmente no disponible', got %q", rec.Body.String())
		}
	})
}

func TestOpsHandler_RequestID_logged(t *testing.T) {
	// Covers [SPEC-003: AC-008, BR-006]
	t.Run("slog contiene request_id", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "closed", nats: true, dbPool: "ok", gemini: "connected"}
		h := asshttp.NewOpsHandler(prov)
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		orig := slog.Default()
		slog.SetDefault(logger)
		t.Cleanup(func() { slog.SetDefault(orig) })

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		req.Header.Set("X-Request-ID", "550e8400-e29b-41d4-a716-446655440001")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "request_id") {
			t.Fatalf("expected slog to contain request_id, got %q", logOut)
		}
		if !strings.Contains(logOut, "550e8400-e29b-41d4-a716-446655440001") {
			t.Fatalf("expected slog to contain provided request_id 550e8400..., got %q", logOut)
		}
	})

	t.Run("genera request_id si no viene header", func(t *testing.T) {
		// Arrange
		prov := &fakeOpsProvider{breakerState: "closed", nats: true, dbPool: "ok", gemini: "connected"}
		h := asshttp.NewOpsHandler(prov)
		var buf bytes.Buffer
		logger := slog.New(slog.NewJSONHandler(&buf, nil))
		orig := slog.Default()
		slog.SetDefault(logger)
		t.Cleanup(func() { slog.SetDefault(orig) })

		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		logOut := buf.String()
		if !strings.Contains(logOut, "request_id") {
			t.Fatalf("expected slog to contain generated request_id, got %q", logOut)
		}
	})
}
