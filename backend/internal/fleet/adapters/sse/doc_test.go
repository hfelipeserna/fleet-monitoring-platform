package sse

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestDocTypes_AlertMsg(t *testing.T) {
	// Covers [SPEC-002: AC-005, BR-006]
	t.Run("AlertMsg fields Seq and Data", func(t *testing.T) {
		// Arrange
		msg := AlertMsg{Seq: 42, Data: []byte(`{"plate":"GTP980"}`)}
		// Act
		seq := msg.Seq
		data := msg.Data
		// Assert
		if seq != 42 { // AC-005 id:seq contract
			t.Fatalf("expected 42, got %d", seq)
		}
		if string(data) != `{"plate":"GTP980"}` { // AC-005 data JSON
			t.Fatalf("expected JSON, got %q", string(data))
		}
		typ := reflect.TypeOf(msg)
		if typ.NumField() != 2 { // contract AlertMsg{Seq,Data}
			t.Fatalf("expected 2 fields, got %d", typ.NumField())
		}
		if f, ok := typ.FieldByName("Seq"); !ok || f.Type.Kind() != reflect.Uint64 {
			t.Fatalf("expected Seq uint64 field")
		}
		if f, ok := typ.FieldByName("Data"); !ok || f.Type.Kind() != reflect.Slice {
			t.Fatalf("expected Data []byte field")
		}
	})
}

func TestDocTypes_PosMsg(t *testing.T) {
	// Covers [SPEC-002: AC-001, BR-006]
	t.Run("PosMsg fields Plate Lat Lon Speed ReceivedAt Data", func(t *testing.T) {
		// Arrange
		lat := 4.711111
		lon := -74.072222
		now := time.Now().UTC()
		msg := PosMsg{Seq: 7, Plate: "GTP980", Lat: &lat, Lon: &lon, Speed: 42, ReceivedAt: now, Data: []byte(`{}`)}
		// Act
		plate := msg.Plate
		speed := msg.Speed
		// Assert
		if plate != "GTP980" { // AC-001 plate contract
			t.Fatalf("expected GTP980, got %q", plate)
		}
		if speed != 42 { // AC-001 speed contract
			t.Fatalf("expected 42, got %d", speed)
		}
		if msg.Seq != 7 { // BR-006 seq contract
			t.Fatalf("expected 7, got %d", msg.Seq)
		}
		if *msg.Lat != 4.711111 { // BR-012 lat contract
			t.Fatalf("lat mismatch")
		}
		if msg.ReceivedAt != now {
			t.Fatalf("ReceivedAt mismatch")
		}
		typ := reflect.TypeOf(msg)
		for _, name := range []string{"Seq", "Plate", "Lat", "Lon", "Speed", "ReceivedAt", "Data"} {
			if _, ok := typ.FieldByName(name); !ok {
				t.Fatalf("expected field %q", name)
			}
		}
	})
}

func TestDocTypes_WithPingInterval(t *testing.T) {
	// Covers [SPEC-002: AC-005, BR-006]
	t.Run("WithPingInterval sets Handler.pingInterval", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil, WithPingInterval(15*time.Millisecond))
		// Act
		handler, ok := h.(*Handler)
		// Assert
		if !ok { // AC-005 WithPingInterval contract
			t.Fatalf("expected *Handler, got %T", h)
		}
		if handler.pingInterval != 15*time.Millisecond { // AC-005 ping 15ms
			t.Fatalf("expected 15ms, got %v", handler.pingInterval)
		}
	})

	t.Run("WithPingInterval 5s sets 5s", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil, WithPingInterval(5*time.Second))
		// Act
		handler := h.(*Handler)
		// Assert
		if handler.pingInterval != 5*time.Second { // BR-006 configurable ping
			t.Fatalf("expected 5s, got %v", handler.pingInterval)
		}
	})

	t.Run("NewHandler nil nil WithPingInterval no panic", func(t *testing.T) {
		// Arrange
		// Act
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic %v", r) // BR-006 no panic
				}
			}()
			_ = NewHandler(nil, nil, WithPingInterval(5*time.Second))
		}()
		// Assert
		// no panic is success
	})

	t.Run("default 15s if 0", func(t *testing.T) {
		// Arrange
		h0 := NewHandler(nil, nil)
		hZero := NewHandler(nil, nil, WithPingInterval(0))
		// Act
		hd0 := h0.(*Handler)
		hdZero := hZero.(*Handler)
		// Assert
		if hd0.pingInterval != 15*time.Second { // AC-005 default 15s
			t.Fatalf("expected default 15s, got %v", hd0.pingInterval)
		}
		if hdZero.pingInterval != 15*time.Second { // BR-006 0 -> default
			t.Fatalf("expected default 15s for zero, got %v", hdZero.pingInterval)
		}
	})

	t.Run("Option type is func(*handlerConfig)", func(t *testing.T) {
		// Arrange
		opt := WithPingInterval(time.Second)
		// Act
		cfg := handlerConfig{}
		opt(&cfg)
		// Assert
		if cfg.pingInterval != time.Second { // BR-006 Option contract
			t.Fatalf("expected 1s, got %v", cfg.pingInterval)
		}
		typ := reflect.TypeOf(opt)
		if typ.Kind() != reflect.Func {
			t.Fatalf("expected func, got %v", typ.Kind())
		}
	})
}

func TestDocTypes_SSEBufferSizeContract(t *testing.T) {
	// Covers [SPEC-002: BR-006, NFR-003]
	t.Run("sseBufferSize is 256", func(t *testing.T) {
		// Arrange
		// Act
		size := sseBufferSize
		// Assert
		if size != 256 { // BR-006 fits 16GB host
			t.Fatalf("expected 256, got %d", size)
		}
	})
}

func TestRound6InSSE(t *testing.T) {
	// Covers [SPEC-002: AC-001, BR-012]
	t.Run("encodeFleet Lat 4.71111119 rounded to 4.711111", func(t *testing.T) {
		// Arrange
		lat := 4.71111119
		lon := -74.07222299
		msg := PosMsg{Seq: 10, Plate: "GTP980", Lat: &lat, Lon: &lon, Speed: 42, ReceivedAt: time.Now().UTC()}
		// Act
		_, _, data := encodeFleet(msg)
		// Assert
		var out struct {
			Lat *float64 `json:"lat"`
			Lon *float64 `json:"lon"`
		}
		if err := json.Unmarshal(data, &out); err != nil { // AC-001 json valid
			t.Fatalf("unmarshal %v data %s", err, string(data))
		}
		if out.Lat == nil || *out.Lat != 4.711111 { // BR-012 Round6 6dec
			t.Fatalf("expected 4.711111, got %v", out.Lat)
		}
		if out.Lon == nil || *out.Lon != -74.072223 { // BR-012 Round6
			t.Fatalf("expected -74.072223, got %v", out.Lon)
		}
		if !containsStr(string(data), "4.711111") { // AC-001 json has 6dec
			t.Fatalf("expected 4.711111 in json %s", string(data))
		}
	})

	t.Run("encodeFleet with Data containing unrounded lat rounds via Data", func(t *testing.T) {
		// Arrange
		raw := map[string]interface{}{"plate": "GTP980", "lat": 4.71111119, "lon": -74.07222299, "speed": 42, "received_at": time.Now().UTC().Format(time.RFC3339Nano)}
		rawData, _ := json.Marshal(raw)
		msg := PosMsg{Seq: 11, Plate: "GTP980", Data: rawData}
		// Act
		_, event, data := encodeFleet(msg)
		// Assert
		if event != "fleet:position" { // AC-001 event
			t.Fatalf("expected fleet:position, got %q", event)
		}
		var out struct {
			Lat *float64 `json:"lat"`
			Lon *float64 `json:"lon"`
		}
		_ = json.Unmarshal(data, &out)
		if out.Lat == nil || *out.Lat != 4.711111 {
			t.Fatalf("expected rounded via Data 4.711111, got %v data %s", out.Lat, string(data))
		}
		if !containsStr(string(data), "4.711111") {
			t.Fatalf("expected rounded lat in data %s", string(data))
		}
	})

	t.Run("handler returns 404 for unknown path", func(t *testing.T) {
		// Arrange
		h := NewHandler(nil, nil)
		req := newHTTPRequest("GET", "/unknown", nil)
		rec := newHTTPRecorder()
		// Act
		h.ServeHTTP(rec, req)
		// Assert
		if rec.Code != http.StatusNotFound { // BR-006 unknown path 404
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

// helpers for last test to avoid import cycle
func newHTTPRequest(method, path string, body interface{}) *http.Request {
	req, _ := http.NewRequest(method, path, nil)
	req.Header.Set("Accept", "text/event-stream")
	return req
}

type httpRecorder struct {
	*testing.T
	header http.Header
	Code   int
	Body   string
}

func newHTTPRecorder() *httpRecorder {
	r := &httpRecorder{header: make(http.Header), Code: 200}
	return r
}

func (r *httpRecorder) Header() http.Header { return r.header }
func (r *httpRecorder) Write(b []byte) (int, error) {
	r.Body += string(b)
	return len(b), nil
}
func (r *httpRecorder) WriteHeader(statusCode int) { r.Code = statusCode }

func containsStr(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
