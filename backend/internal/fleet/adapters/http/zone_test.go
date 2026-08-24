package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	fleethttp "fleetmonitoring/backend/internal/fleet/adapters/http"
	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

type mockZoneService struct {
	createFn func(name string, coords [][]float64) (fleet.Zone, error)
	listFn   func() ([]fleet.Zone, error)
	updateFn func(id string, name string, coords [][]float64) (fleet.Zone, error)
	deleteFn func(id string) error
}

func (m *mockZoneService) CreateZone(name string, coords [][]float64) (fleet.Zone, error) {
	if m.createFn != nil {
		return m.createFn(name, coords)
	}
	return fleet.Zone{}, nil
}

func (m *mockZoneService) ListZones() ([]fleet.Zone, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, nil
}

func (m *mockZoneService) UpdateZone(id string, name string, coords [][]float64) (fleet.Zone, error) {
	if m.updateFn != nil {
		return m.updateFn(id, name, coords)
	}
	return fleet.Zone{}, nil
}

func (m *mockZoneService) DeleteZone(id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return nil
}

func (m *mockZoneService) Create(ctx context.Context, name string, coords [][]float64) (fleet.Zone, error) {
	return m.CreateZone(name, coords)
}

func (m *mockZoneService) List(ctx context.Context) ([]fleet.Zone, error) {
	return m.ListZones()
}

func (m *mockZoneService) Update(ctx context.Context, id string, name string, coords [][]float64) (fleet.Zone, error) {
	return m.UpdateZone(id, name, coords)
}

func (m *mockZoneService) Delete(ctx context.Context, id string) error {
	return m.DeleteZone(id)
}

func buildZoneHandler(svc *mockZoneService) http.Handler {
	// Expected production signature: func NewZoneHandler(svc ZoneService) http.Handler
	// where ZoneService is consumer-side interface with Create/List/Update/Delete + validation
	// This will fail RED until adapters/http/zone.go implements NewZoneHandler.
	return fleethttp.NewZoneHandler(svc)
}

var _ = shared.ParsePlate
var _ = fleet.Zone{}

func validPolygon() [][]float64 {
	return [][]float64{
		{-74.07, 4.71},
		{-74.05, 4.71},
		{-74.05, 4.73},
		{-74.07, 4.73},
		{-74.07, 4.71},
	}
}

func zonePayload(name string, coords [][]float64) []byte {
	m := map[string]any{
		"name": name,
		"geojson": map[string]any{
			"type":        "Polygon",
			"coordinates": [][][]float64{coords},
		},
	}
	b, _ := json.Marshal(m)
	return b
}

// Covers [SPEC-002: AC-003, AC-004, BR-002, BR-005, FR-003]
func TestZonesHandler(t *testing.T) {
	t.Run("POST /api/zones Polygon cerrado 5 coords -> 201 Feature", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, BR-005, FR-003]
		// Arrange
		coords := validPolygon()
		svc := &mockZoneService{
			createFn: func(name string, c [][]float64) (fleet.Zone, error) {
				return fleet.Zone{ID: "550e8400-e29b-41d4-a716-446655440001", Name: name, Coordinates: c}, nil
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.10:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json body, got %q err %v", rec.Body.String(), err)
		}
		if resp["id"] == nil {
			t.Fatalf("expected id UUID field, got %s", rec.Body.String())
		}
		idStr, _ := resp["id"].(string)
		if len(idStr) != 36 {
			t.Fatalf("expected UUID length 36, got %q body %s", idStr, rec.Body.String())
		}
		if resp["name"] != "Zona Norte" {
			t.Fatalf("expected name Zona Norte, got %v body %s", resp["name"], rec.Body.String())
		}
		geom, ok := resp["geometry"].(map[string]any)
		if !ok {
			// also accept geojson wrapper
			if g2, ok2 := resp["geojson"].(map[string]any); ok2 {
				geom = g2
			} else {
				t.Fatalf("expected geometry/geojson Polygon field, got %s", rec.Body.String())
			}
		}
		if geom["type"] != "Polygon" {
			t.Fatalf("expected geometry type Polygon, got %v body %s", geom["type"], rec.Body.String())
		}
		// ST_IsValid and ST_Area>0 are persistence guarantees; HTTP layer must echo valid geojson
		coordsResp, _ := json.Marshal(geom["coordinates"])
		if !strings.Contains(string(coordsResp), "-74.07") {
			t.Fatalf("expected coordinates echo, got %s", string(coordsResp))
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("expected Content-Type application/json, got %q", ct)
		}
	})

	t.Run("POST Polygon no cerrado first!=last -> 400 validation", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.06, 4.72},
		}
		svc := &mockZoneService{
			createFn: func(name string, c [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("polygon not closed: %w", fleet.ErrNotClosed)
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for not closed polygon first!=last, got %d body %s", rec.Code, rec.Body.String())
		}
		bodyStr := strings.ToLower(rec.Body.String())
		if !strings.Contains(bodyStr, "validation") {
			t.Fatalf("expected validation error body, got %s", rec.Body.String())
		}
	})

	t.Run("POST 3 coords -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for 3 coords, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
			t.Fatalf("expected validation details, got %s", rec.Body.String())
		}
	})

	t.Run("POST area 0 colineal 4 coords -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.03, 4.71},
			{-74.07, 4.71},
		}
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for zero area colineal, got %d body %s", rec.Code, rec.Body.String())
		}
		bodyStr := strings.ToLower(rec.Body.String())
		if !strings.Contains(bodyStr, "validation") && !strings.Contains(bodyStr, "area") {
			t.Fatalf("expected validation/area error, got %s", rec.Body.String())
		}
	})

	t.Run("POST 102 coords >101 -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := make([][]float64, 102)
		for i := 0; i < 101; i++ {
			coords[i] = []float64{-74.07 + float64(i)*0.0001, 4.71 + float64(i)*0.0001}
		}
		coords[101] = coords[0]
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for 102 coords >101, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
			t.Fatalf("expected validation for >101 coords, got %s", rec.Body.String())
		}
	})

	t.Run("POST name blank / 101 runes -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := validPolygon()
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		cases := []string{"", "   ", strings.Repeat("a", 101)}
		for _, name := range cases {
			body := zonePayload(name, coords)
			req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			// Act
			h.ServeHTTP(rec, req)

			// Assert
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400 for name %q blank/101 runes, got %d body %s", name, rec.Code, rec.Body.String())
			}
			if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
				t.Fatalf("expected validation for blank name %q, got %s", name, rec.Body.String())
			}
		}
	})

	t.Run("GET /api/zones -> 200 FeatureCollection con zona creada", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-005, FR-003]
		// Arrange
		zone := fleet.Zone{
			ID:          "550e8400-e29b-41d4-a716-446655440002",
			Name:        "Zona Norte",
			Coordinates: validPolygon(),
		}
		svc := &mockZoneService{
			listFn: func() ([]fleet.Zone, error) {
				return []fleet.Zone{zone}, nil
			},
		}
		h := buildZoneHandler(svc)
		req := httptest.NewRequest(http.MethodGet, "/api/zones", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d body %s", rec.Code, rec.Body.String())
		}
		var fc map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &fc); err != nil {
			t.Fatalf("expected FeatureCollection json, got %q err %v", rec.Body.String(), err)
		}
		if fc["type"] != "FeatureCollection" {
			t.Fatalf("expected type FeatureCollection, got %v body %s", fc["type"], rec.Body.String())
		}
		features, ok := fc["features"].([]any)
		if !ok || len(features) != 1 {
			t.Fatalf("expected 1 feature, got %v body %s", fc["features"], rec.Body.String())
		}
		f0, _ := features[0].(map[string]any)
		if f0["type"] != "Feature" {
			t.Fatalf("expected Feature type, got %v", f0["type"])
		}
		if f0["id"] == nil {
			t.Fatalf("expected id UUID in Feature, got %v", f0)
		}
		props, _ := f0["properties"].(map[string]any)
		if props["name"] != "Zona Norte" {
			t.Fatalf("expected properties.name Zona Norte, got %v", props["name"])
		}
		geom, _ := f0["geometry"].(map[string]any)
		if geom["type"] != "Polygon" {
			t.Fatalf("expected geometry Polygon, got %v", geom["type"])
		}
	})

	t.Run("PUT /api/zones/{id} válido -> 200", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		id := "550e8400-e29b-41d4-a716-446655440003"
		newCoords := [][]float64{
			{-74.08, 4.72},
			{-74.06, 4.72},
			{-74.06, 4.74},
			{-74.08, 4.74},
			{-74.08, 4.72},
		}
		svc := &mockZoneService{
			updateFn: func(uid string, name string, coords [][]float64) (fleet.Zone, error) {
				if uid != id {
					t.Fatalf("expected id %q, got %q", id, uid)
				}
				return fleet.Zone{ID: id, Name: name, Coordinates: coords}, nil
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte v2", newCoords)
		req := httptest.NewRequest(http.MethodPut, "/api/zones/"+id, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for valid PUT, got %d body %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("expected json 200 body, got %q err %v", rec.Body.String(), err)
		}
		if resp["id"] != id {
			t.Fatalf("expected id %q echo, got %v body %s", id, resp["id"], rec.Body.String())
		}
	})

	t.Run("PUT id random UUID -> 404", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		randomID := "550e8400-e29b-41d4-a716-446655440099"
		svc := &mockZoneService{
			updateFn: func(id string, name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("not found: %w", fleet.ErrNotFound)
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte v2", validPolygon())
		req := httptest.NewRequest(http.MethodPut, "/api/zones/"+randomID, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for random UUID PUT, got %d body %s", rec.Code, rec.Body.String())
		}
		bodyStr := strings.ToLower(rec.Body.String())
		if !strings.Contains(bodyStr, "not_found") && !strings.Contains(bodyStr, "not found") {
			t.Fatalf("expected not_found error payload, got %s", rec.Body.String())
		}
	})

	t.Run("PUT geo inválido -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		id := "550e8400-e29b-41d4-a716-446655440003"
		invalidCoords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Norte v2", invalidCoords)
		req := httptest.NewRequest(http.MethodPut, "/api/zones/"+id, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for PUT geo inválido, got %d body %s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "validation") {
			t.Fatalf("expected validation 400 body, got %s", rec.Body.String())
		}
	})

	t.Run("DELETE /api/zones/{id} -> 204", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		id := "550e8400-e29b-41d4-a716-446655440004"
		svc := &mockZoneService{
			deleteFn: func(uid string) error {
				if uid != id {
					t.Fatalf("expected delete id %q, got %q", id, uid)
				}
				return nil
			},
		}
		h := buildZoneHandler(svc)
		req := httptest.NewRequest(http.MethodDelete, "/api/zones/"+id, nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusNoContent {
			t.Fatalf("expected 204 for DELETE, got %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Body.Len() != 0 {
			t.Fatalf("expected empty body for 204, got %q", rec.Body.String())
		}
	})

	t.Run("DELETE random UUID -> 404", func(t *testing.T) {
		// Covers [SPEC-002: AC-004, BR-002, FR-003]
		// Arrange
		randomID := "550e8400-e29b-41d4-a716-446655440099"
		svc := &mockZoneService{
			deleteFn: func(id string) error {
				return fmt.Errorf("zone not found: %w", fleet.ErrNotFound)
			},
		}
		h := buildZoneHandler(svc)
		req := httptest.NewRequest(http.MethodDelete, "/api/zones/"+randomID, nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for random UUID DELETE, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("POST without ST_IsValid self-intersect bowtie -> 400", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002, FR-003]
		// Arrange
		coords := [][]float64{
			{-74.07, 4.71},
			{-74.05, 4.73},
			{-74.07, 4.73},
			{-74.05, 4.71},
			{-74.07, 4.71},
		}
		svc := &mockZoneService{}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona Bowtie", coords)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for self-intersect bowtie ST_IsValid false, got %d body %s", rec.Code, rec.Body.String())
		}
		bodyStr := strings.ToLower(rec.Body.String())
		if !strings.Contains(bodyStr, "validation") && !strings.Contains(bodyStr, "intersection") && !strings.Contains(bodyStr, "valid") {
			t.Fatalf("expected validation/self-intersection error, got %s", rec.Body.String())
		}
	})

	t.Run("rate limit 10/min IP -> 429 Retry-After:5 tras 11 POST rápidos", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-005, FR-003]
		// Arrange
		svc := &mockZoneService{
			createFn: func(name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{ID: "550e8400-e29b-41d4-a716-446655440010", Name: name, Coordinates: coords}, nil
			},
		}
		h := buildZoneHandler(svc)
		coords := validPolygon()
		var lastRec *httptest.ResponseRecorder
		// Act: 11 POST rápidos desde misma IP
		for i := 0; i < 11; i++ {
			body := zonePayload(fmt.Sprintf("Zona %d", i), coords)
			req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "10.0.0.1:1234"
			// X-Forwarded-For also considered if behind LB; test both
			req.Header.Set("X-Forwarded-For", "10.0.0.1")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			lastRec = rec
			if i < 10 && rec.Code == http.StatusTooManyRequests {
				t.Fatalf("expected not rate limited on request %d, got 429 too early body %s", i+1, rec.Body.String())
			}
		}

		// Assert: 11th must be 429 with Retry-After:5
		if lastRec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429 after 11 POST rápidos, got %d body %s", lastRec.Code, lastRec.Body.String())
		}
		if got := lastRec.Header().Get("Retry-After"); got != "5" {
			t.Fatalf("expected Retry-After:5, got %q body %s", got, lastRec.Body.String())
		}
		bodyStr := strings.ToLower(lastRec.Body.String())
		if !strings.Contains(bodyStr, "rate") && !strings.Contains(bodyStr, "limit") {
			t.Fatalf("expected rate_limited payload, got %s", lastRec.Body.String())
		}
	})

	t.Run("POST duplicate name -> 409 (crea Norte, luego POST mismo name distinto geom -> 409)", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002] 409
		// Arrange
		coords1 := validPolygon()
		coords2 := [][]float64{
			{-74.06, 4.72},
			{-74.04, 4.72},
			{-74.04, 4.74},
			{-74.06, 4.74},
			{-74.06, 4.72},
		}
		svc := &mockZoneService{
			createFn: func(name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("duplicate zone name: %w", errors.Join(shared.ErrValidation, fleet.ErrDuplicateZoneName, fmt.Errorf("critical_zones_name_unique")))
			},
		}
		h := buildZoneHandler(svc)
		// first create would be 201 but we simulate duplicate on second POST
		body := zonePayload("Norte", coords2)
		_ = coords1
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.11:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 for duplicate name POST, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Fatalf("expected no Retry-After for 409, got %q", got)
		}
		var payload map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("expected json body, got %q", rec.Body.String())
		}
		if payload["error"] != "zone name already exists" {
			t.Fatalf("expected error zone name already exists, got %v body %s", payload["error"], rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Fatalf("expected json content-type, got %q", ct)
		}
	})

	t.Run("PUT duplicate name -> 409 (crea Zona A y B, PUT B con name de A -> 409)", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002] 409
		// Arrange
		idB := "550e8400-e29b-41d4-a716-446655440030"
		svc := &mockZoneService{
			updateFn: func(id string, name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("duplicate zone name: %w", errors.Join(shared.ErrValidation, fleet.ErrDuplicateZoneName, fmt.Errorf("critical_zones_name_unique")))
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("Zona A", validPolygon())
		req := httptest.NewRequest(http.MethodPut, "/api/zones/"+idB, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.12:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 for duplicate name PUT, got %d body %s", rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Retry-After"); got != "" {
			t.Fatalf("expected no Retry-After for 409 PUT, got %q", got)
		}
		var payload map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
			t.Fatalf("expected json, got %q", rec.Body.String())
		}
		if payload["error"] != "zone name already exists" {
			t.Fatalf("expected zone name already exists, got %v", payload["error"])
		}
	})

	t.Run("POST duplicate name case-insensitive -> 409", func(t *testing.T) {
		// Covers [SPEC-002: AC-003, BR-002] 409
		// Arrange
		svc := &mockZoneService{
			createFn: func(name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, fmt.Errorf("duplicate zone name: %w", errors.Join(shared.ErrValidation, fleet.ErrDuplicateZoneName, fmt.Errorf("lower(name) duplicate")))
			},
		}
		h := buildZoneHandler(svc)
		body := zonePayload("norte", validPolygon())
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "192.168.1.13:1234"
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409 case-insensitive, got %d body %s", rec.Code, rec.Body.String())
		}
	})
}
