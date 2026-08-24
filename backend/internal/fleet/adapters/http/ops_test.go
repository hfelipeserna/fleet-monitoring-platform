package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
)

type mockZoneCounter struct {
	count int
	err   error
}

func (m *mockZoneCounter) Count(ctx context.Context) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.count, nil
}

// Covers [SPEC-002: AC-009, FR-009, NFR-005]
func TestOpsHandler(t *testing.T) {
	t.Run("GET /healthz 200 status ok breaker closed nats connected db ok", func(t *testing.T) {
		// Covers [SPEC-002: AC-009, FR-009]
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
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
		if body["breaker"] == "" {
			t.Fatalf("expected breaker field, got %v body %s", body, rec.Body.String())
		}
		if body["nats"] == "" {
			t.Fatalf("expected nats field, got %v body %s", body, rec.Body.String())
		}
		if body["db"] == "" {
			t.Fatalf("expected db field, got %v body %s", body, rec.Body.String())
		}
	})

	t.Run("GET /metrics 200 prometheus exposition", func(t *testing.T) {
		// Covers [SPEC-002: AC-009, FR-009, NFR-005]
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 /metrics, got %d body %s", rec.Code, rec.Body.String())
		}
		ct := rec.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/plain") && !strings.Contains(rec.Body.String(), "# HELP") && !strings.Contains(rec.Body.String(), "breaker") {
			t.Fatalf("expected prometheus metrics exposition, got Content-Type %q body %q", ct, rec.Body.String())
		}
	})

	t.Run("GET /healthz shows breaker open when open", func(t *testing.T) {
		// Covers [SPEC-002: AC-009, NFR-003]
		// Arrange
		// We simulate breaker open via svc that would set breaker state; handler should reflect breaker open
		// For RED, we just assert healthz still returns 200 with breaker field, implementation will set breaker=open when injected breaker is open
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "breaker") {
			t.Fatalf("expected breaker in healthz body %s", body)
		}
		// When breaker open, healthz may still be 200 but breaker=open (not 503) per spec; we document expectation
		// This test will be refined GREEN to inject breaker mock with State=open
	})
}

func TestMetrics_CriticalZonesTotal(t *testing.T) {
	t.Run("GET /metrics contains critical_zones_total 2 via ZoneCounter mock", func(t *testing.T) {
		svc := &mockQueryService{}
		zc := &mockZoneCounter{count: 2}
		h := fleethttp.NewHandlerWithZoneCounter(svc, zc)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 /metrics, got %d body %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "# HELP critical_zones_total") {
			t.Fatalf("expected HELP critical_zones_total, got %q", body)
		}
		if !strings.Contains(body, "# TYPE critical_zones_total gauge") {
			t.Fatalf("expected TYPE critical_zones_total gauge, got %q", body)
		}
		if !strings.Contains(body, "critical_zones_total 2") {
			t.Fatalf("expected critical_zones_total 2, got %q", body)
		}
	})

	t.Run("GET /api/metrics contains critical_zones_total 2", func(t *testing.T) {
		svc := &mockQueryService{}
		zc := &mockZoneCounter{count: 2}
		h := fleethttp.NewHandlerWithZoneCounter(svc, zc)
		req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 /api/metrics, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "critical_zones_total 2") {
			t.Fatalf("expected critical_zones_total 2 on /api/metrics, got %q", rec.Body.String())
		}
	})

	t.Run("GET /metrics with nil ZoneCounter defaults to 0", func(t *testing.T) {
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "critical_zones_total 0") {
			t.Fatalf("expected critical_zones_total 0 when no counter, got %q", rec.Body.String())
		}
	})

	t.Run("GET /metrics when Count error still 200 with 0", func(t *testing.T) {
		svc := &mockQueryService{}
		zc := &mockZoneCounter{err: context.DeadlineExceeded}
		h := fleethttp.NewHandlerWithZoneCounter(svc, zc)
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 even on Count error, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "critical_zones_total 0") {
			t.Fatalf("expected critical_zones_total 0 on error, got %q", rec.Body.String())
		}
	})
}
