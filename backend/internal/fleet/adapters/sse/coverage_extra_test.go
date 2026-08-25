package sse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Covers [SPEC-002: NFR-005]

type mockAlertSub struct {
	ch  chan AlertMsg
	err error
}

func (m *mockAlertSub) SubscribeAlerts(ctx context.Context, lastSeq uint64) (<-chan AlertMsg, func(), error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	if m.ch == nil {
		ch := make(chan AlertMsg)
		close(ch)
		return ch, func() {}, nil
	}
	return m.ch, func() {}, nil
}

type mockPosSub struct {
	ch  chan PosMsg
	err error
}

func (m *mockPosSub) SubscribePositions(ctx context.Context, plate *string, lastSeq uint64) (<-chan PosMsg, func(), error) {
	if m.err != nil {
		return nil, nil, m.err
	}
	if m.ch == nil {
		ch := make(chan PosMsg)
		close(ch)
		return ch, func() {}, nil
	}
	return m.ch, func() {}, nil
}

func TestCoverage_SSE(t *testing.T) {
	t.Run("parseLastSeq empty valid invalid overflow", func(t *testing.T) {
		// Arrange
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		// empty
		// Act
		v, err := parseLastSeq(req)
		// Assert
		if err != nil || v != 0 {
			t.Fatalf("expected 0 nil, got %d %v", v, err)
		}
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.Header.Set("Last-Event-ID", "5")
		v2, err2 := parseLastSeq(req2)
		if err2 != nil || v2 != 6 {
			t.Fatalf("expected 6, got %d %v", v2, err2)
		}
		req3 := httptest.NewRequest(http.MethodGet, "/", nil)
		req3.Header.Set("Last-Event-ID", "bad")
		_, err3 := parseLastSeq(req3)
		if err3 == nil {
			t.Fatalf("expected error bad")
		}
		req4 := httptest.NewRequest(http.MethodGet, "/", nil)
		req4.Header.Set("Last-Event-ID", "18446744073709551615")
		_, err4 := parseLastSeq(req4)
		if err4 == nil {
			t.Fatalf("expected overflow")
		}
	})

	t.Run("validateAccept true false", func(t *testing.T) {
		// Arrange
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Accept", "text/event-stream")
		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.Header.Set("Accept", "application/json")
		// Act
		a := validateAccept(req)
		b := validateAccept(req2)
		// Assert
		if !a || b {
			t.Fatalf("expected true false, got %v %v", a, b)
		}
	})

	t.Run("handleAlerts nil -> 503", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil).(*Handler)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d body %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("handleAlerts subscribe error -> 503", func(t *testing.T) {
		// Arrange
		mock := &mockAlertSub{err: errFakeSSE}
		h := NewHandler(mock, nil).(*Handler)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503, got %d", rec.Code)
		}
	})

	t.Run("handleFleet invalid plate -> 400 and nil positions ->503", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil).(*Handler)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream?plate=BAD", nil)
		req.Header.Set("Accept", "text/event-stream")
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid plate, got %d", rec.Code)
		}
		// nil positions
		h2 := NewHandler(nil, nil).(*Handler)
		req2 := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream", nil)
		req2.Header.Set("Accept", "text/event-stream")
		rec2 := httptest.NewRecorder()
		h2.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusServiceUnavailable {
			t.Fatalf("expected 503 nil positions, got %d", rec2.Code)
		}
	})

	t.Run("encodeFleet with Data and with LatLon", func(t *testing.T) {
		// Arrange
		lat := 4.71111119
		lon := -74.07222229
		data := json.RawMessage(`{"plate":"ABC123","lat":4.71111119,"lon":-74.07222229,"speed":10,"received_at":"2026-08-24T10:00:00Z"}`)
		m := PosMsg{Seq: 1, Data: data}
		m2 := PosMsg{Seq: 2, Plate: "ABC123", Lat: &lat, Lon: &lon, Speed: 10, ReceivedAt: time.Now().UTC()}
		// Act
		_, _, d1 := encodeFleet(m)
		_, _, d2 := encodeFleet(m2)
		// Assert
		if len(d1) == 0 || len(d2) == 0 {
			t.Fatalf("expected data")
		}
		var out map[string]interface{}
		_ = json.Unmarshal(d1, &out)
		if out["plate"] != "ABC123" {
			t.Fatalf("expected plate, got %v", out)
		}
	})

	t.Run("ServeHTTP not found", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
		rec := httptest.NewRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("serveStream accept missing ->400 and Last-Event-ID invalid ->400", func(t *testing.T) {
		// Arrange
		h := NewHandler(&mockAlertSub{}, nil).(*Handler)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		// no Accept
		rec := httptest.NewRecorder()
		// Act
		h.handleAlerts(rec, req)
		// Assert
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 accept missing, got %d", rec.Code)
		}
		// invalid Last-Event-ID
		req2 := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req2.Header.Set("Accept", "text/event-stream")
		req2.Header.Set("Last-Event-ID", "bad")
		rec2 := httptest.NewRecorder()
		h.handleAlerts(rec2, req2)
		if rec2.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid Last-Event-ID, got %d", rec2.Code)
		}
	})
}

var errFakeSSE = errFakeSSEErr{}
type errFakeSSEErr struct{}
func (e errFakeSSEErr) Error() string { return "fake" }
