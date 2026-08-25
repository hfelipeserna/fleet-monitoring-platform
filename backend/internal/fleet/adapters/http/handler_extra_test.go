package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-002: AC-001, AC-002, AC-008, AC-009, BR-003, BR-006, BR-007, FR-001, FR-002]

type fakeOpsExtra struct {
	breaker string
	nats    bool
	db      string
}

func (f *fakeOpsExtra) BreakerState() string { return f.breaker }
func (f *fakeOpsExtra) NATSConnected() bool  { return f.nats }
func (f *fakeOpsExtra) DBPoolStat() string   { return f.db }

type fakeZoneCounterExtra struct {
	n   int
	err error
}

func (f *fakeZoneCounterExtra) Count(ctx context.Context) (int, error) {
	return f.n, f.err
}

func TestFleetHandler_ExtraOpsAndBreaker(t *testing.T) {
	// Covers [SPEC-002: AC-009, FR-009]
	t.Run("WithOps nil keeps default closed true ok", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc, fleethttp.WithOps(nil))
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["breaker"] != "closed" {
			t.Fatalf("expected breaker closed default, got %q body %s", body["breaker"], rec.Body.String())
		}
		if body["nats"] != "connected" {
			t.Fatalf("expected nats connected default, got %q", body["nats"])
		}
	})

	t.Run("WithOps non-nil overrides BreakerState to open", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		ops := &fakeOpsExtra{breaker: "open", nats: false, db: "total=1 idle=0"}
		h := fleethttp.NewHandler(svc, fleethttp.WithOps(ops))
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 for breaker open, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "5" {
			t.Fatalf("expected Retry-After:5, got %q", got)
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["breaker"] != "open" {
			t.Fatalf("expected breaker open, got %q", body["breaker"])
		}
		if body["nats"] != "disconnected" {
			t.Fatalf("expected nats disconnected, got %q", body["nats"])
		}
		if body["db"] != "total=1 idle=0" {
			t.Fatalf("expected db stat, got %q", body["db"])
		}
		if body["status"] != "degraded" {
			t.Fatalf("expected status degraded, got %q", body["status"])
		}
	})

	t.Run("NewHandlerWithZoneCounter with ops and api healthz prefix", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		zc := &fakeZoneCounterExtra{n: 3}
		ops := &fakeOpsExtra{breaker: "closed", nats: true, db: "ok"}
		h := fleethttp.NewHandlerWithZoneCounter(svc, zc, ops)
		req := httptest.NewRequest(http.MethodGet, "/api/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["breaker"] != "closed" {
			t.Fatalf("expected closed, got %q", body["breaker"])
		}
	})

	t.Run("NewHandlerWithZoneCounter nil ops uses default", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		zc := &fakeZoneCounterExtra{n: 0}
		h := fleethttp.NewHandlerWithZoneCounter(svc, zc, nil)
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("WithZoneCounter fluent returns same handler", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		fh, ok := h.(*fleethttp.Handler)
		if !ok {
			t.Fatalf("expected *Handler type")
		}
		zc := &fakeZoneCounterExtra{n: 5}

		// Act
		ret := fh.WithZoneCounter(zc)

		// Assert
		if ret != fh {
			t.Fatalf("expected fluent WithZoneCounter to return same handler")
		}
		// Also serve metrics should now reflect count 5
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "critical_zones_total 5") {
			t.Fatalf("expected critical_zones_total 5, got %q", rec.Body.String())
		}
	})
}

func TestFleetHandler_Positions_ErrorBranches(t *testing.T) {
	t.Run("querier returns validation -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("limit out of range: %w", shared.ErrValidation)
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
			t.Fatalf("expected validation body, got %s", rec.Body.String())
		}
	})

	t.Run("querier returns internal -> 500", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("db down: %w", fmt.Errorf("connection refused"))
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "internal") {
			t.Fatalf("expected internal error body, got %s", rec.Body.String())
		}
	})

	t.Run("POST to positions -> 405", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid plate format -> 400 contains validation", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?plate=bad!!", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("limit empty defaults and limit invalid string -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if limit != 100 {
					t.Fatalf("expected default limit 100, got %d", limit)
				}
				return []fleet.VehiclePos{}, "", nil
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 default limit, got %d body %s", rec.Code, rec.Body.String())
		}

		// Arrange second: invalid limit string
		svc2 := &mockQueryService{}
		h2 := fleethttp.NewHandler(svc2)
		req2 := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=abc", nil)
		rec2 := httptest.NewRecorder()

		// Act
		h2.ServeHTTP(rec2, req2)

		// Assert
		if rec2.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for limit abc, got %d body %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("cursor invalid base64 -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?cursor=!!!not-base64", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 cursor invalid, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success with next_cursor empty -> null and with value", func(t *testing.T) {
		// Arrange
		base := time.Now().UTC()
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{{Plate: "ABC123", Speed: 0, ReceivedAt: base, Status: "idle"}}, "", nil
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		var body map[string]json.RawMessage
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if string(body["next_cursor"]) != "null" {
			t.Fatalf("expected next_cursor null when empty, got %s body %s", string(body["next_cursor"]), rec.Body.String())
		}
		// second with cursor
		svc2 := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{{Plate: "ABC123", Speed: 10, ReceivedAt: base, Status: "moving"}}, "abc123cursor==", nil
			},
		}
		h2 := fleethttp.NewHandler(svc2)
		req2 := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec2 := httptest.NewRecorder()
		h2.ServeHTTP(rec2, req2)
		var body2 map[string]any
		_ = json.Unmarshal(rec2.Body.Bytes(), &body2)
		if body2["next_cursor"] != "abc123cursor==" {
			t.Fatalf("expected next_cursor abc123cursor==, got %v body %s", body2["next_cursor"], rec2.Body.String())
		}
	})
}

func TestFleetHandler_History_ErrorBranches(t *testing.T) {
	t.Run("invalid plate path -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/BAD/history", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("from after to -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?from=2026-08-24T11:00:00Z&to=2026-08-24T10:00:00Z", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 from after to, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid from RFC3339 -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?from=not-time", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid from, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid to RFC3339 -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?to=not-time", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid to, got %d", rec.Code)
		}
	})

	t.Run("limit out of range -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?limit=0", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 limit 0, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("cursor invalid -> 400", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?cursor=bad!!", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 cursor bad, got %d", rec.Code)
		}
	})

	t.Run("querier validation -> 400 and internal -> 500", func(t *testing.T) {
		// Arrange validation
		svc := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("validation: %w", shared.ErrValidation)
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?limit=5", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 validation, got %d body %s", rec.Code, rec.Body.String())
		}
		// Arrange internal
		svc2 := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("db down")
			},
		}
		h2 := fleethttp.NewHandler(svc2)
		req2 := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?limit=5", nil)
		rec2 := httptest.NewRecorder()

		// Act
		h2.ServeHTTP(rec2, req2)

		// Assert
		if rec2.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 internal, got %d body %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("method not allowed and not found prefix", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodPost, "/api/vehicles/GTP890/history", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 POST history, got %d", rec.Code)
		}
		// Not found when path does not end with /history
		req2 := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/other", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for other suffix, got %d body %s", rec2.Code, rec2.Body.String())
		}
	})

	t.Run("RFC3339 without nanos succeeds", func(t *testing.T) {
		// Arrange
		svc := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if from == nil || to == nil {
					t.Fatalf("expected from/to parsed")
				}
				return []fleet.VehiclePos{}, "", nil
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?from=2026-08-24T10:00:00Z&to=2026-08-24T11:00:00Z&limit=5", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 RFC3339 without nanos, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("success returns points and next_cursor string", func(t *testing.T) {
		// Arrange
		base := time.Now().UTC()
		svc := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{{Plate: "GTP890", Speed: 10, ReceivedAt: base, Status: "moving"}}, "next123", nil
			},
		}
		h := fleethttp.NewHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP890/history?limit=5", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if _, ok := body["points"]; !ok {
			t.Fatalf("expected points field, got %s", rec.Body.String())
		}
		if body["next_cursor"] != "next123" {
			t.Fatalf("expected next_cursor next123, got %v", body["next_cursor"])
		}
	})
}

func TestFleetHandler_MetricsAndHealthz(t *testing.T) {
	t.Run("healthz breaker half-open still 200 but status ok", func(t *testing.T) {
		// Arrange
		ops := &fakeOpsExtra{breaker: "half-open", nats: true, db: "ok"}
		h := fleethttp.NewHandler(&mockQueryService{}, fleethttp.WithOps(ops))
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 half-open not degraded, got %d body %s", rec.Code, rec.Body.String())
		}
		var body map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["breaker"] != "half-open" {
			t.Fatalf("expected half-open, got %q", body["breaker"])
		}
		if body["status"] != "ok" {
			t.Fatalf("expected ok, got %q", body["status"])
		}
	})

	t.Run("api/metrics same as metrics with zoneCounter error returns 0", func(t *testing.T) {
		// Arrange
		zc := &fakeZoneCounterExtra{n: 0, err: fmt.Errorf("db down")}
		ops := &fakeOpsExtra{breaker: "open", nats: true, db: "ok"}
		h := fleethttp.NewHandler(&mockQueryService{}, fleethttp.WithZoneCounter(zc), fleethttp.WithOps(ops))
		req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "breaker_state 0") {
			t.Fatalf("expected breaker_state 0 for open, got %q", body)
		}
		if !strings.Contains(body, "critical_zones_total 0") {
			t.Fatalf("expected critical_zones_total 0 on error, got %q", body)
		}
	})

	t.Run("metrics breaker closed val 1 and zones 7", func(t *testing.T) {
		// Arrange
		zc := &fakeZoneCounterExtra{n: 7}
		ops := &fakeOpsExtra{breaker: "closed", nats: true, db: "ok"}
		h := fleethttp.NewHandler(&mockQueryService{}, fleethttp.WithZoneCounter(zc), fleethttp.WithOps(ops))
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "breaker_state 1") {
			t.Fatalf("expected breaker_state 1, got %q", body)
		}
		if !strings.Contains(body, "critical_zones_total 7") {
			t.Fatalf("expected 7, got %q", body)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
			t.Fatalf("expected text/plain, got %q", ct)
		}
	})

	t.Run("metrics without zoneCounter nil -> 0", func(t *testing.T) {
		// Arrange
		h := fleethttp.NewHandler(&mockQueryService{})
		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "critical_zones_total 0") {
			t.Fatalf("expected 0 count, got %q", rec.Body.String())
		}
	})
}
