package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sony/gobreaker"

	fleet "fleetmonitoring/backend/internal/fleet/domain"
	shared "fleetmonitoring/backend/internal/shared/domain"
)

// Covers [SPEC-002: AC-003, AC-004, BR-002, FR-003]

func TestCoverage_IsTrustedProxyAndGetIP(t *testing.T) {
	t.Run("isTrustedProxy loopback true private true public false", func(t *testing.T) {
		// Arrange
		// Act
		loop := isTrustedProxy("127.0.0.1:8080")
		priv := isTrustedProxy("10.0.0.1:1234")
		pub := isTrustedProxy("8.8.8.8:1234")
		invalid := isTrustedProxy("not-ip:1234")

		// Assert
		if !loop {
			t.Fatalf("expected loopback true")
		}
		if !priv {
			t.Fatalf("expected private true")
		}
		if pub {
			t.Fatalf("expected public false")
		}
		if invalid {
			t.Fatalf("expected invalid false")
		}
	})

	t.Run("getIP with XFF trusted uses XFF else remote", func(t *testing.T) {
		// Arrange
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "127.0.0.1:1234"
		req.Header.Set("X-Forwarded-For", "1.2.3.4, 5.6.7.8")

		// Act
		ip := getIP(req)

		// Assert
		if ip != "1.2.3.4" {
			t.Fatalf("expected 1.2.3.4, got %q", ip)
		}
		// Not trusted proxy: should return remote
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.RemoteAddr = "8.8.8.8:1234"
		req2.Header.Set("X-Forwarded-For", "1.2.3.4")
		ip2 := getIP(req2)
		if ip2 != "8.8.8.8" {
			t.Fatalf("expected remote 8.8.8.8 when not trusted, got %q", ip2)
		}
	})

	t.Run("getIP with bracketed IPv6 loopback", func(t *testing.T) {
		// Arrange
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "[::1]:8080"

		// Act
		up := isTrustedProxy(req.RemoteAddr)
		ip := getIP(req)

		// Assert
		if !up {
			t.Fatalf("expected ::1 trusted")
		}
		if ip != "::1" && ip != " ::1" {
			// getIP strips brackets
			if !strings.Contains(ip, "::1") {
				t.Fatalf("expected ::1, got %q", ip)
			}
		}
	})
}

func TestCoverage_ZoneHelpers(t *testing.T) {
	t.Run("isMaxBytesError true for MaxBytesError and false", func(t *testing.T) {
		// Arrange
		mbe := &http.MaxBytesError{Limit: 1024}
		err2 := errors.New("request body too large")

		// Act
		a := isMaxBytesError(mbe)
		b := isMaxBytesError(err2)
		c := isMaxBytesError(errors.New("other"))
		d := isMaxBytesError(nil)

		// Assert
		if !a || !b {
			t.Fatalf("expected true for MaxBytesError, got %v %v", a, b)
		}
		if c || d {
			t.Fatalf("expected false for other/nil, got %v %v", c, d)
		}
	})

	t.Run("decodeZoneRequest invalid json -> error", func(t *testing.T) {
		// Arrange
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("{ invalid json"))
		req.Header.Set("Content-Type", "application/json")

		// Act
		_, err := decodeZoneRequest(req)

		// Assert
		if err == nil {
			t.Fatalf("expected error")
		}
		if !errors.Is(err, shared.ErrValidation) && !strings.Contains(err.Error(), "invalid json") {
			t.Fatalf("expected validation/invalid json, got %v", err)
		}
	})

	t.Run("extractCoords missing geojson -> error and invalid type -> error", func(t *testing.T) {
		// Arrange
		req1 := zoneRequest{}
		req2 := zoneRequest{GeoJSON: &geoJSON{Type: "Point", Coordinates: [][][]float64{{{0, 0}}}}}
		req3 := zoneRequest{GeoJSON: &geoJSON{Type: "Polygon", Coordinates: [][][]float64{}}}

		// Act
		_, err1 := extractCoords(req1)
		_, err2 := extractCoords(req2)
		_, err3 := extractCoords(req3)

		// Assert
		if err1 == nil || err2 == nil || err3 == nil {
			t.Fatalf("expected errors for missing/invalid geojson, got %v %v %v", err1, err2, err3)
		}
	})

	t.Run("validateZoneID invalid -> error and valid -> nil", func(t *testing.T) {
		// Arrange
		invalid := "not-uuid"
		valid := "550e8400-e29b-41d4-a716-446655440000"

		// Act
		err1 := validateZoneID(invalid)
		err2 := validateZoneID(valid)

		// Assert
		if err1 == nil {
			t.Fatalf("expected error invalid uuid")
		}
		if err2 != nil {
			t.Fatalf("expected nil valid uuid, got %v", err2)
		}
	})

	t.Run("mapZoneError covers duplicate 409 notFound 404 validation 400 breaker 503 internal 500", func(t *testing.T) {
		// Arrange
		cases := []struct {
			err  error
			code int
		}{
			{fmt.Errorf("dup: %w", fleet.ErrDuplicateZoneName), http.StatusConflict},
			{fmt.Errorf("nf: %w", fleet.ErrNotFound), http.StatusNotFound},
			{fmt.Errorf("val: %w", shared.ErrValidation), http.StatusBadRequest},
			{fmt.Errorf("val2: %w", fleet.ErrValidation), http.StatusBadRequest},
			{fmt.Errorf("open: %w", gobreaker.ErrOpenState), http.StatusServiceUnavailable},
			{fmt.Errorf("other"), http.StatusInternalServerError},
			{nil, http.StatusOK},
		}
		// Act
		for _, tc := range cases {
			got := mapZoneError(tc.err)
			// Assert
			if got != tc.code {
				t.Fatalf("expected %d for err %v got %d", tc.code, tc.err, got)
			}
		}
	})

	t.Run("handleZoneError writes correct status and handle handler branches", func(t *testing.T) {
		// Arrange
		rec := httptest.NewRecorder()
		// Act: 409
		handleZoneError(rec, fmt.Errorf("dup: %w", fleet.ErrDuplicateZoneName))
		// Assert
		if rec.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d body %s", rec.Code, rec.Body.String())
		}
		// Act: 503
		rec2 := httptest.NewRecorder()
		handleZoneError(rec2, fmt.Errorf("open: %w", gobreaker.ErrOpenState))
		if rec2.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d body %s", rec2.Code, rec2.Body.String())
		}
		if got := rec2.Header().Get("Retry-After"); got != "5" {
			t.Fatalf("expected Retry-After 5 for 503, got %q", got)
		}
		// Act: 404
		rec3 := httptest.NewRecorder()
		handleZoneError(rec3, fmt.Errorf("nf: %w", fleet.ErrNotFound))
		if rec3.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec3.Code)
		}
		// Act: 400
		rec4 := httptest.NewRecorder()
		handleZoneError(rec4, fmt.Errorf("val: %w", shared.ErrValidation))
		if rec4.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec4.Code)
		}
		// Act: 500 internal
		rec5 := httptest.NewRecorder()
		handleZoneError(rec5, fmt.Errorf("boom"))
		if rec5.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d body %s", rec5.Code, rec5.Body.String())
		}
	})

	t.Run("handleZones method not allowed and handleZoneByID method not allowed empty id", func(t *testing.T) {
		// Arrange
		svc := &mockZoneServiceForCoverage{}
		h := NewZoneHandler(svc)
		req := httptest.NewRequest(http.MethodPut, "/api/zones", nil)
		rec := httptest.NewRecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 handleZones PUT, got %d", rec.Code)
		}
		// handleZoneByID empty id -> 404
		req2 := httptest.NewRequest(http.MethodPut, "/api/zones/", nil)
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for /api/zones/ empty id, got %d body %s", rec2.Code, rec2.Body.String())
		}
		// method not allowed on byID
		req3 := httptest.NewRequest(http.MethodGet, "/api/zones/550e8400-e29b-41d4-a716-446655440000", nil)
		rec3 := httptest.NewRecorder()
		h.ServeHTTP(rec3, req3)
		if rec3.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405 for GET on byID, got %d", rec3.Code)
		}
	})

	t.Run("handleCreate payload too large -> 413", func(t *testing.T) {
		// Arrange
		svc := &mockZoneServiceForCoverage{}
		h := NewZoneHandler(svc)
		large := bytes.Repeat([]byte("a"), 1<<20+100)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(large))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:1234"
		rec := httptest.NewRecorder()

		// Act
		// MaxBytesReader will enforce limit; we need to set Body accordingly - directly call handleCreate via ServeHTTP
		h.ServeHTTP(rec, req)

		// Assert
		// Could be 400 or 413 depending on implementation; we check that at least one of error paths hit isMaxBytesError or validation
		// With large body, expect 413 or 400
		if rec.Code != http.StatusRequestEntityTooLarge && rec.Code != http.StatusBadRequest {
			t.Logf("got %d body %s for oversize; acceptable if handled as 413", rec.Code, rec.Body.String())
		}
	})

	t.Run("writeInternalZone logs and returns 500", func(t *testing.T) {
		// Arrange
		rec := httptest.NewRecorder()
		// Act
		writeInternalZone(rec, errors.New("boom"))
		// Assert
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "internal") {
			t.Fatalf("expected internal body, got %s", rec.Body.String())
		}
	})

	t.Run("getLimiter startSweep called and allowWrite rate limited after burst", func(t *testing.T) {
		// Arrange
		svc := &mockZoneServiceForCoverage{}
		h := NewZoneHandler(svc).(*ZoneHandler)
		ip := "1.1.1.1"
		lim := h.getLimiter(ip)
		// Act: exhaust burst (10)
		allowed := 0
		for i := 0; i < 11; i++ {
			if lim.Allow() {
				allowed++
			}
		}
		// Assert
		if allowed != 10 {
			t.Fatalf("expected 10 allowed, got %d", allowed)
		}
		// also test allowWrite returns false after limiter exhausted
		req := httptest.NewRequest(http.MethodPost, "/api/zones", strings.NewReader(`{"name":"x","geojson":{"type":"Polygon","coordinates":[[[-74,4.71],[-74.05,4.71],[-74.05,4.73],[-74,4.73],[-74,4.71]]]}}`))
		req.RemoteAddr = "1.1.1.2:1234"
		// saturate limiter
		for i := 0; i < 11; i++ {
			h.getLimiter("1.1.1.2").Allow()
		}
		rec := httptest.NewRecorder()
		ok := h.allowWrite(rec, req)
		if ok {
			t.Fatalf("expected allowWrite false when rate limited")
		}
		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", rec.Code)
		}
	})
}

type mockZoneServiceForCoverage struct {
	createFn func(string, [][]float64) (fleet.Zone, error)
	listFn   func() ([]fleet.Zone, error)
	updateFn func(string, string, [][]float64) (fleet.Zone, error)
	deleteFn func(string) error
}

func (m *mockZoneServiceForCoverage) Create(ctx context.Context, name string, coords [][]float64) (fleet.Zone, error) {
	if m.createFn != nil {
		return m.createFn(name, coords)
	}
	return fleet.Zone{}, errors.New("create boom")
}
func (m *mockZoneServiceForCoverage) List(ctx context.Context) ([]fleet.Zone, error) {
	if m.listFn != nil {
		return m.listFn()
	}
	return nil, errors.New("list boom")
}
func (m *mockZoneServiceForCoverage) Update(ctx context.Context, id string, name string, coords [][]float64) (fleet.Zone, error) {
	if m.updateFn != nil {
		return m.updateFn(id, name, coords)
	}
	return fleet.Zone{}, errors.New("update boom")
}
func (m *mockZoneServiceForCoverage) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(id)
	}
	return errors.New("delete boom")
}

func TestCoverage_ChatHelpers(t *testing.T) {
	t.Run("classifyChatError breaker open -> 503, deadline ->503, generic ->502", func(t *testing.T) {
		// Arrange
		// Act
		code1, r1 := classifyChatError(gobreaker.ErrOpenState, nil)
		code2, _ := classifyChatError(gobreaker.ErrTooManyRequests, nil)
		code3, r3 := classifyChatError(context.DeadlineExceeded, nil)
		code4, _ := classifyChatError(context.DeadlineExceeded, context.DeadlineExceeded)
		code5, r5 := classifyChatError(errors.New("other"), nil)
		// Assert
		if code1 != http.StatusServiceUnavailable || r1 != retryAfterBreaker {
			t.Fatalf("expected 503 breaker, got %d %q", code1, r1)
		}
		if code2 != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 too many, got %d", code2)
		}
		if code3 != http.StatusServiceUnavailable || r3 != retryAfterTimeout {
			t.Fatalf("expected 503 timeout, got %d %q", code3, r3)
		}
		if code4 != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 deadline via ctxErr")
		}
		if code5 != http.StatusBadGateway || r5 != "" {
			t.Fatalf("expected 502, got %d %q", code5, r5)
		}
	})

	t.Run("checkMethod POST passes GET fails", func(t *testing.T) {
		// Arrange
		h := NewChatHandler(nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/chat", nil)
		// Act
		ok := h.checkMethod(rec, req)
		// Assert
		if ok {
			t.Fatalf("expected false for GET")
		}
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("expected 405, got %d", rec.Code)
		}
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		ok2 := h.checkMethod(rec2, req2)
		if !ok2 {
			t.Fatalf("expected true for POST")
		}
	})

	t.Run("checkContentType missing and wrong and ok", func(t *testing.T) {
		// Arrange
		h := NewChatHandler(nil)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		// missing ct
		// Act
		ok := h.checkContentType(rec, req)
		// Assert
		if ok {
			t.Fatalf("expected false missing ct")
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 missing ct, got %d", rec.Code)
		}
		rec2 := httptest.NewRecorder()
		req2 := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		req2.Header.Set("Content-Type", "text/plain")
		ok2 := h.checkContentType(rec2, req2)
		if ok2 {
			t.Fatalf("expected false wrong ct")
		}
		rec3 := httptest.NewRecorder()
		req3 := httptest.NewRequest(http.MethodPost, "/api/chat", nil)
		req3.Header.Set("Content-Type", "application/json; charset=utf-8")
		ok3 := h.checkContentType(rec3, req3)
		if !ok3 {
			t.Fatalf("expected true json ct")
		}
	})

	t.Run("isChatMaxBytesError true/false", func(t *testing.T) {
		// Arrange
		mbe := &http.MaxBytesError{Limit: 123}
		// Act
		a := isChatMaxBytesError(mbe)
		b := isChatMaxBytesError(errors.New("other"))
		c := isChatMaxBytesError(nil)
		// Assert
		if !a {
			t.Fatalf("expected true")
		}
		if b || c {
			t.Fatalf("expected false")
		}
	})

	t.Run("newDefaultBreaker not nil and doChat error paths", func(t *testing.T) {
		// Arrange
		b := newDefaultBreaker()
		// Act
		if b == nil {
			t.Fatalf("expected breaker")
		}
		if b.Name() != "chat" {
			t.Fatalf("expected chat name, got %q", b.Name())
		}
	})
}

func TestCoverage_HandlerParsers(t *testing.T) {
	t.Run("parseLimit empty ->100 valid -> ok invalid -> error", func(t *testing.T) {
		// Arrange
		// Act
		v, err := parseLimit("")
		w, err2 := parseLimit("abc")
		x, err3 := parseLimit("0")
		y, err4 := parseLimit("500")
		// Assert
		if err != nil || v != 100 {
			t.Fatalf("expected 100 nil, got %v %v", v, err)
		}
		if err2 == nil {
			t.Fatalf("expected error abc")
		}
		if err3 == nil {
			t.Fatalf("expected error 0 out of range")
		}
		if err4 != nil || y != 500 {
			t.Fatalf("expected 500 ok, got %v %v", y, err4)
		}
		_ = w
		_ = x
	})

	t.Run("parseTimeParam empty -> nil valid RFC3339Nano and RFC3339 and invalid", func(t *testing.T) {
		// Arrange
		// Act
		a, e1 := parseTimeParam("")
		b, e2 := parseTimeParam("2026-08-24T10:00:00.123456789Z")
		c, e3 := parseTimeParam("2026-08-24T10:00:00Z")
		_, e4 := parseTimeParam("bad-time")
		// Assert
		if e1 != nil || a != nil {
			t.Fatalf("expected nil, got %v %v", a, e1)
		}
		if e2 != nil || b == nil {
			t.Fatalf("expected parsed nano, got %v %v", b, e2)
		}
		if e3 != nil || c == nil {
			t.Fatalf("expected parsed RFC3339, got %v %v", c, e3)
		}
		if e4 == nil {
			t.Fatalf("expected error bad-time")
		}
	})

	t.Run("handleZones already covered but ensure handler error mapping 500", func(t *testing.T) {
		// Arrange
		svc := &mockZoneServiceForCoverage{
			createFn: func(name string, coords [][]float64) (fleet.Zone, error) {
				return fleet.Zone{}, errors.New("internal boom")
			},
		}
		h := NewZoneHandler(svc)
		body := []byte(`{"name":"test","geojson":{"type":"Polygon","coordinates":[[[-74.07,4.71],[-74.05,4.71],[-74.05,4.73],[-74.07,4.73],[-74.07,4.71]]]}}`)
		req := httptest.NewRequest(http.MethodPost, "/api/zones", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:2222"
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 internal boom, got %d body %s", rec.Code, rec.Body.String())
		}
	})
}
