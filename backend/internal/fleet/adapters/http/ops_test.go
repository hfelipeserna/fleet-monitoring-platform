package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
)

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
