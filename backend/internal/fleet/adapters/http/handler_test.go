package http_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
	"fleetmonitoring/backend/internal/fleet/application"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type mockQueryService struct {
	lastPositionsFn func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error)
	historyFn       func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error)
}

func (m *mockQueryService) LastPositions(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if m.lastPositionsFn != nil {
		return m.lastPositionsFn(ctx, plate, limit, cursor)
	}
	return nil, "", nil
}

func (m *mockQueryService) History(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
	if m.historyFn != nil {
		return m.historyFn(ctx, plate, from, to, limit, cursor)
	}
	return nil, "", nil
}

func f64(v float64) *float64 { return &v }

func buildFleetHandler(svc *mockQueryService) http.Handler {
	// Expected: func NewHandler(svc *application.QueryService) http.Handler
	// We adapt via application.NewQueryService + fleethttp.NewHandler
	// For RED, this will fail until production implements NewHandler with correct signature.
	// Use type assertion to keep compile-time dependency on fleethttp.NewHandler
	return fleethttp.NewHandler(svc)
}

// Helper to assert coverage: use application.QueryService type to force import usage
var _ = application.NewQueryService
var _ = shared.ParsePlate
var _ = fleet.VehiclePos{}

func TestFleetHandler(t *testing.T) {
	t.Run("GET /api/fleet/positions?limit=2 200 + next_cursor + vehicles[] con status", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, FR-001]
		// Arrange
		base := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		vehicles := []fleet.VehiclePos{
			{Plate: "ABC123", Lat: f64(4.71111119), Lon: f64(-74.07222229), Speed: 10, ReceivedAt: base.Add(3 * time.Minute), Status: "moving"},
			{Plate: "GTP890", Lat: f64(4.72222222), Lon: f64(-74.08222222), Speed: 0, ReceivedAt: base.Add(2 * time.Minute), Status: "idle"},
		}
		next := base64.StdEncoding.EncodeToString(fmt.Appendf(nil, "%s|%s", "GTP890", base.Add(2*time.Minute).Format(time.RFC3339Nano)))
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if limit != 2 {
					t.Fatalf("expected limit 2, got %d", limit)
				}
				return vehicles, next, nil
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]json.RawMessage
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json body, got %q err %v", rec.Body.String(), err)
		}
		if _, ok := resp["vehicles"]; !ok {
			t.Fatalf("expected vehicles field, got %s", rec.Body.String())
		}
		if _, ok := resp["next_cursor"]; !ok {
			t.Fatalf("expected next_cursor field, got %s", rec.Body.String())
		}
		var body struct {
			Vehicles []map[string]any `json:"vehicles"`
			NextCursor *string `json:"next_cursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal vehicles failed %v body %s", err, rec.Body.String())
		}
		if len(body.Vehicles) != 2 {
			t.Fatalf("expected 2 vehicles, got %d body %s", len(body.Vehicles), rec.Body.String())
		}
		for _, v := range body.Vehicles {
			if _, ok := v["status"]; !ok {
				t.Fatalf("expected status field in vehicle %v", v)
			}
			if _, ok := v["plate"]; !ok {
				t.Fatalf("expected plate field %v", v)
			}
			if _, has := v["client_event_id"]; has {
				t.Fatalf("PII leak client_event_id should be filtered %v", v)
			}
		}
		if body.NextCursor == nil || *body.NextCursor != next {
			t.Fatalf("expected next_cursor %q got %v body %s", next, body.NextCursor, rec.Body.String())
		}
	})

	t.Run("GET /api/fleet/positions?plate=GTP980 200 solo ese", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-012, FR-001]
		// Arrange
		vehicles := []fleet.VehiclePos{
			{Plate: "GTP980", Lat: f64(4.71), Lon: f64(-74.07), Speed: 42, ReceivedAt: time.Now().UTC(), Status: "moving"},
		}
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if plate == nil || string(*plate) != "GTP980" {
					t.Fatalf("expected plate GTP980 filter, got %v", plate)
				}
				return vehicles, "", nil
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?plate=GTP980", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Vehicles []struct{ Plate string `json:"plate"` } `json:"vehicles"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal failed %v", err)
		}
		if len(body.Vehicles) != 1 || body.Vehicles[0].Plate != "GTP980" {
			t.Fatalf("expected solo GTP980, got %v body %s", body.Vehicles, rec.Body.String())
		}
	})

	t.Run("GET /api/fleet/positions?plate=GTP98 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-007, FR-001]
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("plate invalid: %w", shared.ErrValidation)
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?plate=GTP98", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for plate GTP98 invalid, got %d body %s", rec.Code, rec.Body.String())
		}
		body := strings.ToLower(rec.Body.String())
		if !strings.Contains(body, "validation") {
			t.Fatalf("expected validation error body, got %s", rec.Body.String())
		}
	})

	t.Run("GET /api/fleet/positions?cursor=bad 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-003, BR-007, FR-001]
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if cursor == "bad" {
					return nil, "", fmt.Errorf("cursor decode failed: %w", shared.ErrValidation)
				}
				return nil, "", nil
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?cursor=bad", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for cursor=bad, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("GET /api/vehicles/GTP890/history?from=&to=&limit=5 200 points DESC", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-003, FR-002]
		// Arrange
		base := time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC)
		points := []fleet.VehiclePos{
			{Plate: "GTP890", Lat: f64(4.71), Lon: f64(-74.07), Speed: 40, ReceivedAt: base.Add(50 * time.Minute), Status: "moving"},
			{Plate: "GTP890", Lat: f64(4.72), Lon: f64(-74.08), Speed: 30, ReceivedAt: base.Add(40 * time.Minute), Status: "moving"},
			{Plate: "GTP890", Lat: f64(4.73), Lon: f64(-74.09), Speed: 20, ReceivedAt: base.Add(30 * time.Minute), Status: "idle"},
			{Plate: "GTP890", Lat: f64(4.74), Lon: f64(-74.10), Speed: 10, ReceivedAt: base.Add(20 * time.Minute), Status: "idle"},
			{Plate: "GTP890", Lat: f64(4.75), Lon: f64(-74.11), Speed: 0, ReceivedAt: base.Add(10 * time.Minute), Status: "idle"},
		}
		from := base.Format(time.RFC3339)
		to := base.Add(time.Hour).Format(time.RFC3339)
		svc := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, fromT, toT *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				if string(plate) != "GTP890" {
					t.Fatalf("expected plate GTP890, got %q", plate)
				}
				if limit != 5 {
					t.Fatalf("expected limit 5, got %d", limit)
				}
				if fromT == nil || toT == nil {
					t.Fatalf("expected from/to not nil")
				}
				return points, "", nil
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/vehicles/GTP890/history?from=%s&to=%s&limit=5", from, to), nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var body struct {
			Points []struct {
				ReceivedAt time.Time `json:"received_at"`
				Speed int `json:"speed"`
			} `json:"points"`
			NextCursor *string `json:"next_cursor"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal points failed %v body %s", err, rec.Body.String())
		}
		if len(body.Points) != 5 {
			t.Fatalf("expected 5 points, got %d body %s", len(body.Points), rec.Body.String())
		}
		for i := 1; i < len(body.Points); i++ {
			if body.Points[i].ReceivedAt.After(body.Points[i-1].ReceivedAt) {
				t.Fatalf("expected DESC order, index %d %v after %v", i, body.Points[i].ReceivedAt, body.Points[i-1].ReceivedAt)
			}
		}
	})

	t.Run("GET /api/vehicles/GTP89/history 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-002, BR-007, FR-002]
		// Arrange
		svc := &mockQueryService{
			historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return nil, "", fmt.Errorf("plate invalid: %w", shared.ErrValidation)
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/vehicles/GTP89/history?from=2026-08-24T10:00:00Z&to=2026-08-24T11:00:00Z&limit=5", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for plate GTP89 invalid, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("payload sin client_event_id y lat/lon 6 dec", func(t *testing.T) {
		// Covers [SPEC-002: AC-008, BR-010, FR-008]
		// Arrange
		svc := &mockQueryService{
			lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
				return []fleet.VehiclePos{
					{Plate: "GTP980", Lat: f64(4.71111119), Lon: f64(-74.07222229), Speed: 42, ReceivedAt: time.Now().UTC(), Status: "moving"},
					{Plate: "GTP981", Lat: nil, Lon: nil, Speed: 0, ReceivedAt: time.Now().UTC(), Status: "idle"},
				}, "", nil
			},
		}
		h := buildFleetHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions?limit=2", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		bodyStr := rec.Body.String()
		if strings.Contains(strings.ToLower(bodyStr), "client_event_id") {
			t.Fatalf("PII leak: payload must not contain client_event_id, got %s", bodyStr)
		}
		var body struct {
			Vehicles []struct {
				Plate string `json:"plate"`
				Lat *float64 `json:"lat"`
				Lon *float64 `json:"lon"`
			} `json:"vehicles"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal failed %v body %s", err, bodyStr)
		}
		if body.Vehicles[0].Lat == nil || math.Abs(*body.Vehicles[0].Lat-4.711111) > 1e-6 {
			t.Fatalf("expected lat 6 dec 4.711111, got %v body %s", body.Vehicles[0].Lat, bodyStr)
		}
		if body.Vehicles[0].Lon == nil || math.Abs(*body.Vehicles[0].Lon-(-74.072222)) > 1e-6 {
			t.Fatalf("expected lon 6 dec -74.072222, got %v body %s", body.Vehicles[0].Lon, bodyStr)
		}
		if body.Vehicles[1].Lat != nil || body.Vehicles[1].Lon != nil {
			t.Fatalf("expected nullable lat/lon preserved nil for second vehicle, got lat %v lon %v body %s", body.Vehicles[1].Lat, body.Vehicles[1].Lon, bodyStr)
		}
	})

	t.Run("healthz/metrics stub -> breaker/nats/db ok", func(t *testing.T) {
		// Covers [SPEC-002: AC-009, FR-009, NFR-005]
		// Arrange
		svc := &mockQueryService{}
		h := buildFleetHandler(svc)

		// Act healthz
		req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		// Assert healthz
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 healthz, got %d body %s", rec.Code, rec.Body.String())
		}
		var hz map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &hz); err != nil {
			t.Fatalf("healthz not json %q err %v", rec.Body.String(), err)
		}
		for _, k := range []string{"status", "breaker", "nats", "db"} {
			if _, ok := hz[k]; !ok {
				t.Fatalf("expected healthz field %q, got %v body %s", k, k, rec.Body.String())
			}
		}
		if hz["status"] != "ok" {
			t.Fatalf("expected status ok, got %v body %s", hz["status"], rec.Body.String())
		}

		// Act metrics
		req2 := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)

		// Assert metrics
		if rec2.Code != http.StatusOK {
			t.Fatalf("expected 200 metrics, got %d body %s", rec2.Code, rec2.Body.String())
		}
		metricsBody := rec2.Body.String()
		if !strings.Contains(metricsBody, "breaker") && !strings.Contains(metricsBody, "api_sse") && !strings.Contains(metricsBody, "p95") {
			t.Fatalf("expected metrics to expose breaker/api_sse/p95, got %q", metricsBody)
		}
	})
}

// Additional validation table for query params: ensures handler validates RFC3339 and limit range
func TestFleetHandler_ValidationTable(t *testing.T) {
	cases := []struct {
		name string
		path string
	}{
		{"limit 0 -> 400", "/api/fleet/positions?limit=0"},
		{"limit 501 -> 400", "/api/fleet/positions?limit=501"},
		{"history from>to -> 400", "/api/vehicles/GTP890/history?from=2026-08-24T11:00:00Z&to=2026-08-24T10:00:00Z&limit=5"},
		{"history from invalid RFC3339 -> 400", "/api/vehicles/GTP890/history?from=not-rfc3339&to=2026-08-24T11:00:00Z&limit=5"},
		{"history plate invalid -> 400", "/api/vehicles/GTP89/history?from=2026-08-24T10:00:00Z&to=2026-08-24T11:00:00Z&limit=5"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// Covers [SPEC-002: AC-001, AC-002, BR-007]
			// Arrange
			svc := &mockQueryService{
				lastPositionsFn: func(ctx context.Context, plate *shared.Plate, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
					return nil, "", fmt.Errorf("validation: %w", shared.ErrValidation)
				},
				historyFn: func(ctx context.Context, plate shared.Plate, from, to *time.Time, limit int, cursor string) ([]fleet.VehiclePos, string, error) {
					return nil, "", fmt.Errorf("validation: %w", shared.ErrValidation)
				},
			}
			h := buildFleetHandler(svc)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for %q, got %d body %s", tc.name, rec.Code, rec.Body.String())
			}
		})
	}
}
