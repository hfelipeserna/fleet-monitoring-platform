package sse_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	sse "fleetmonitoring/backend/internal/fleet/adapters/sse"
)

// Covers [SPEC-002: AC-005/006/001, BR-006]
// Step5 RED: suite TDD para SSE fleet:position + alert:critical con heartbeat, Last-Event-ID y backpressure.
// Esta suite FALLA intencionalmente hasta que exista adapters/sse/handler.go
// con NewHandler / NewAlertHandler / NewFleetHandler, interfaces consumer-side y WithPingInterval (TDD RED).

// ---- consumer-side interfaces esperadas (documentan contrato GREEN) ----
// Expected GREEN signatures (definidas en sse/handler.go, consumer-side):
//   type AlertMsg struct { Seq uint64; Data []byte }
//   type PosMsg struct { Seq uint64; Plate string; Lat *float64; Lon *float64; Speed int; ReceivedAt time.Time }
//   type AlertSubscriber interface { SubscribeAlerts(ctx context.Context, lastSeq uint64) (<-chan AlertMsg, func(), error) }
//   type TelemetrySubscriber interface { SubscribePositions(ctx context.Context, plate *string, lastSeq uint64) (<-chan PosMsg, func(), error) }
//   func NewHandler(alerts AlertSubscriber, positions TelemetrySubscriber, opts ...Option) http.Handler
//   func WithPingInterval(d time.Duration) Option
// Si tu implementación separa en NewAlertHandler / NewFleetHandler, adapta build helpers manteniendo contrato de ping/replay/filtro.

// ---- fakes ----

type fakeAlertSubscriber struct {
	ch            chan sse.AlertMsg
	lastSeq       uint64
	err           error
	unsubCalled   bool
	subscribeCalls int
}

func (f *fakeAlertSubscriber) SubscribeAlerts(ctx context.Context, lastSeq uint64) (<-chan sse.AlertMsg, func(), error) {
	// Arrange helper: registra lastSeq para verificar replay Last-Event-ID
	f.lastSeq = lastSeq
	f.subscribeCalls++
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.ch == nil {
		f.ch = make(chan sse.AlertMsg, 10)
	}
	return f.ch, func() { f.unsubCalled = true }, nil
}

type fakeTelemetrySubscriber struct {
	ch            chan sse.PosMsg
	plateFilter   *string
	lastSeq       uint64
	err           error
	unsubCalled   bool
	subscribeCalls int
}

func (f *fakeTelemetrySubscriber) SubscribePositions(ctx context.Context, plate *string, lastSeq uint64) (<-chan sse.PosMsg, func(), error) {
	f.plateFilter = plate
	f.lastSeq = lastSeq
	f.subscribeCalls++
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.ch == nil {
		f.ch = make(chan sse.PosMsg, 10)
	}
	return f.ch, func() { f.unsubCalled = true }, nil
}

// sseRecorder envuelve httptest.ResponseRecorder e implementa http.Flusher de forma thread-safe
type sseRecorder struct {
	*httptest.ResponseRecorder
	mu         sync.Mutex
	flushed    bool
	flushCount int
}

func newSSERecorder() *sseRecorder {
	return &sseRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (r *sseRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Write(b)
}

func (r *sseRecorder) WriteHeader(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ResponseRecorder.WriteHeader(code)
}

func (r *sseRecorder) Header() http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Header()
}

func (r *sseRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ResponseRecorder.Body.String()
}

func (r *sseRecorder) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flushed = true
	r.flushCount++
}

// helpers de construcción: compilan solo cuando sse.NewHandler exista
func buildAlertHandler(alerts *fakeAlertSubscriber, opts ...sse.Option) http.Handler {
	// Expected: func NewHandler(AlertSubscriber, TelemetrySubscriber, ...Option) http.Handler
	// o func NewAlertHandler(AlertSubscriber, ...Option) http.Handler
	// Este helper fuerza dependencia compile-time con sse.NewHandler
	return sse.NewHandler(alerts, nil, opts...)
}

func buildFleetHandler(positions *fakeTelemetrySubscriber, opts ...sse.Option) http.Handler {
	return sse.NewHandler(nil, positions, opts...)
}

func buildCombinedHandler(alerts *fakeAlertSubscriber, positions *fakeTelemetrySubscriber, opts ...sse.Option) http.Handler {
	return sse.NewHandler(alerts, positions, opts...)
}

var _ = sse.WithPingInterval

func f64(v float64) *float64 { return &v }

// Covers [SPEC-002: AC-005/006/001, BR-006]
func TestSSEHandler_AcceptHeader(t *testing.T) {
	// Covers [SPEC-002: AC-005/006, BR-006, FR-006]
	t.Run("Accept sin text/event-stream ->400 header falta", func(t *testing.T) {
		// Covers [SPEC-002: AC-006, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		// sin Accept
		rec := newSSERecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest { // AC-006
			t.Fatalf("expected 400 without Accept text/event-stream, got %d body %s", rec.Code, rec.String())
		}
		body := strings.ToLower(rec.String())
		if !strings.Contains(body, "accept") && !strings.Contains(body, "text/event-stream") {
			t.Fatalf("expected error about Accept header, got %q", rec.String()) // AC-006
		}
	})

	t.Run("Accept application/json ->400 con error", func(t *testing.T) {
		// Covers [SPEC-002: AC-006, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "application/json")
		rec := newSSERecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest { // AC-006
			t.Fatalf("expected 400 for Accept application/json, got %d body %s", rec.Code, rec.String())
		}
	})

	t.Run("fleet stream Accept sin text/event-stream ->400", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-006]
		// Arrange
		positions := &fakeTelemetrySubscriber{ch: make(chan sse.PosMsg, 1)}
		h := buildFleetHandler(positions)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream", nil)
		rec := newSSERecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusBadRequest { // AC-001 BR-006
			t.Fatalf("expected 400 for fleet stream without Accept, got %d body %s", rec.Code, rec.String())
		}
	})
}

// Covers [SPEC-002: AC-005, BR-006, NFR-001]
func TestSSEHandler_AlertCritical(t *testing.T) {
	// Covers [SPEC-002: AC-005, BR-006]
	t.Run("NATS msg -> event: alert:critical id:seq data JSON <2s y Flusher.Flush llamado", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 2)}
		// ping largo para no interferir con assert de mensaje
		h := buildAlertHandler(alerts, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() {
			h.ServeHTTP(rec, req)
			close(done)
		}()
		// dar tiempo a que handler suscriba
		time.Sleep(20 * time.Millisecond)
		payload := map[string]any{"plate": "GTP980", "alert_type": "zone_enter", "zone_id": "550e8400-e29b-41d4-a716-446655440002", "lat": 4.711111, "lon": -74.072222, "speed": 42}
		data, _ := json.Marshal(payload)
		msg := sse.AlertMsg{Seq: 123, Data: data}

		// Act
		start := time.Now()
		alerts.ch <- msg
		// esperar <2s a que se escriba SSE (NFR-001 p95 <2s)
		deadline := time.After(2 * time.Second)
		var body string
		ticked := false
		for !ticked {
			select {
			case <-deadline:
				t.Fatalf("timeout waiting for SSE data <2s")
			default:
				body = rec.String()
				if strings.Contains(body, "event: alert:critical") && strings.Contains(body, "id: 123") {
					ticked = true
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
		}
		elapsed := time.Since(start)
		cancel()
		<-done

		// Assert
		if elapsed > 2*time.Second { // AC-005 NFR-001 <2s
			t.Fatalf("expected SSE latency <2s, got %v body %q", elapsed, body)
		}
		if !strings.Contains(body, "id: 123\n") && !strings.Contains(body, "id: 123\r\n") { // AC-005 id:seq
			t.Fatalf("expected id: 123 in SSE, got %q", body)
		}
		if !strings.Contains(body, "event: alert:critical") { // AC-005 event: alert:critical
			t.Fatalf("expected event: alert:critical, got %q", body)
		}
		if !strings.Contains(body, "data: ") { // AC-005 data: JSON
			t.Fatalf("expected data: JSON, got %q", body)
		}
		// data debe ser JSON válido con plate/alert_type
		lines := strings.Split(body, "\n")
		var dataLine string
		for _, l := range lines {
			if strings.HasPrefix(l, "data: ") {
				dataLine = strings.TrimPrefix(l, "data: ")
				break
			}
		}
		var got map[string]any
		if err := json.Unmarshal([]byte(dataLine), &got); err != nil {
			t.Fatalf("data not JSON %q err %v body %q", dataLine, err, body) // AC-005
		}
		if got["plate"] != "GTP980" {
			t.Fatalf("expected plate GTP980 in data, got %v body %q", got["plate"], body) // AC-005
		}
		if !rec.flushed || rec.flushCount == 0 { // AC-005 Flusher.Flush llamado
			t.Fatalf("expected Flusher.Flush called, flushed=%v count=%d body %q", rec.flushed, rec.flushCount, body)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
			t.Fatalf("expected Content-Type text/event-stream, got %q", ct) // BR-006
		}
	})
}

// Covers [SPEC-002: AC-001, BR-012, BR-006]
func TestSSEHandler_FleetPosition(t *testing.T) {
	// Covers [SPEC-002: AC-001, BR-012]
	t.Run("fleet:position sin plate -> todos", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-012]
		// Arrange
		positions := &fakeTelemetrySubscriber{ch: make(chan sse.PosMsg, 5)}
		h := buildFleetHandler(positions, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()
		time.Sleep(20 * time.Millisecond)
		// Act: publicar 2 placas distintas, sin filtro debe recibir ambos
		now := time.Now().UTC()
		positions.ch <- sse.PosMsg{Seq: 10, Plate: "GTP980", Lat: f64(4.71), Lon: f64(-74.07), Speed: 42, ReceivedAt: now}
		positions.ch <- sse.PosMsg{Seq: 11, Plate: "TTY423", Lat: f64(4.72), Lon: f64(-74.08), Speed: 30, ReceivedAt: now}
		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done
		body := rec.String()

		// Assert
		if positions.plateFilter != nil { // AC-001 BR-012 sin plate = todos => nil filter
			t.Fatalf("expected nil plate filter for all, got %v", *positions.plateFilter)
		}
		if !strings.Contains(body, "GTP980") || !strings.Contains(body, "TTY423") { // AC-001 todos
			t.Fatalf("expected both plates without filter, got %q", body)
		}
		if !strings.Contains(body, "event: fleet:position") { // AC-001 event fleet:position
			t.Fatalf("expected event: fleet:position, got %q", body)
		}
	})

	t.Run("fleet:position con ?plate=GTP980 -> filtro solo ese", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-012]
		// Arrange
		positions := &fakeTelemetrySubscriber{ch: make(chan sse.PosMsg, 5)}
		h := buildFleetHandler(positions, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream?plate=GTP980", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()
		time.Sleep(20 * time.Millisecond)
		now := time.Now().UTC()
		positions.ch <- sse.PosMsg{Seq: 12, Plate: "GTP980", Lat: f64(4.71), Lon: f64(-74.07), Speed: 42, ReceivedAt: now}
		// Simular que NATS solo entrega GTP980 cuando hay filtro; si handler filtra, TTY423 no debe aparecer aunque se intente enviar
		// Para test, solo enviamos GTP980 y verificamos filtro registrado
		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done
		body := rec.String()

		// Assert
		if positions.plateFilter == nil || *positions.plateFilter != "GTP980" { // AC-001 BR-012 con plate filtro
			t.Fatalf("expected plate filter GTP980, got %v", positions.plateFilter)
		}
		if !strings.Contains(body, "GTP980") { // AC-001 solo ese
			t.Fatalf("expected GTP980 in filtered stream, got %q", body)
		}
		if strings.Contains(body, "TTY423") {
			t.Fatalf("expected filtered stream without TTY423, got %q", body) // BR-012
		}
		if !strings.Contains(body, "id: 12") {
			t.Fatalf("expected id: 12, got %q", body) // BR-006
		}
	})
}

// Covers [SPEC-002: AC-005/006, BR-006, NFR-003]
func TestSSEHandler_HeartbeatAndReplay(t *testing.T) {
	t.Run(":ping 15s via WithPingInterval 15ms", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts, sse.WithPingInterval(15*time.Millisecond))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()

		// Act: esperar ping sin mensajes
		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done
		body := rec.String()

		// Assert
		if !strings.Contains(body, ":ping") { // AC-005 BR-006 :ping 15s
			t.Fatalf("expected :ping heartbeat, got %q", body)
		}
		// debe ser formato ":ping\n\n"
		if !strings.Contains(body, ":ping\n\n") && !strings.Contains(body, ":ping\r\n") {
			t.Fatalf("expected :ping\\n\\n format, got %q", body) // BR-006
		}
		if rec.flushCount == 0 {
			t.Fatalf("expected Flush for ping, got %d", rec.flushCount) // BR-006
		}
	})

	t.Run("Last-Event-ID 100 -> replay 101..102 startSeq=101", func(t *testing.T) {
		// Covers [SPEC-002: AC-006, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 5)}
		h := buildAlertHandler(alerts, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Last-Event-ID", "100")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()
		time.Sleep(20 * time.Millisecond)
		// Simular replay: handler debió suscribir con lastSeq=100 y entrega 101..102
		// Enviamos 101..102 como si fueran replay desde NATS
		data1, _ := json.Marshal(map[string]any{"plate": "GTP980", "alert_type": "speeding_on"})
		data2, _ := json.Marshal(map[string]any{"plate": "GTP980", "alert_type": "speeding_off"})
		alerts.ch <- sse.AlertMsg{Seq: 101, Data: data1}
		alerts.ch <- sse.AlertMsg{Seq: 102, Data: data2}
		time.Sleep(50 * time.Millisecond)
		cancel()
		<-done
		body := rec.String()

		// Assert
		if alerts.lastSeq != 101 && alerts.lastSeq != 100 { // AC-006 parse Last-Event-ID 100 -> start 101
			// algunos implementan lastSeq=100 y luego entregan desde 101; aceptamos 100 o 101
			t.Fatalf("expected subscriber called with startSeq 101 (or 100), got %d body %q", alerts.lastSeq, body)
		}
		if !strings.Contains(body, "id: 101") || !strings.Contains(body, "id: 102") { // AC-006 replay 101..102
			t.Fatalf("expected replay ids 101 and 102, got %q lastSeq=%d", body, alerts.lastSeq)
		}
	})
}

// Covers [SPEC-002: AC-006, BR-006, NFR-003]
func TestSSEHandler_NATSDownAndCancel(t *testing.T) {
	t.Run("NATS down ->503 retry:5000 + Retry-After:5", func(t *testing.T) {
		// Covers [SPEC-002: AC-006, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{err: context.DeadlineExceeded}
		// Simular NATS no disponible con error
		alerts.err = context.DeadlineExceeded
		// alternativa: usar error genérico de NATS
		alertsNonNil := &fakeTelemetrySubscriber{err: http.ErrServerClosed}
		_ = alertsNonNil
		fakeErr := &fakeAlertSubscriber{err: http.ErrHandlerTimeout}
		// para test determinístico, usamos error que handler mapea a 503
		fakeErr.err = context.DeadlineExceeded
		h := buildAlertHandler(fakeErr)
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		rec := newSSERecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable { // AC-006 NATS down ->503
			t.Fatalf("expected 503 when NATS down, got %d body %s", rec.Code, rec.String())
		}
		if rec.Header().Get("Retry-After") != "5" { // AC-006 Retry-After:5
			t.Fatalf("expected Retry-After:5, got %q body %s", rec.Header().Get("Retry-After"), rec.String())
		}
		body := rec.String()
		if !strings.Contains(body, "retry: 5000") && !strings.Contains(body, "retry:5000") { // AC-006 retry:5000
			t.Fatalf("expected retry:5000 in body, got %q", body)
		}
		_ = alerts
	})

	t.Run("NATS down fleet ->503", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-006]
		// Arrange
		positions := &fakeTelemetrySubscriber{err: context.DeadlineExceeded}
		h := buildFleetHandler(positions)
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream?plate=GTP980", nil)
		req.Header.Set("Accept", "text/event-stream")
		rec := newSSERecorder()

		// Act
		h.ServeHTTP(rec, req)

		// Assert
		if rec.Code != http.StatusServiceUnavailable { // AC-001 NATS down 503
			t.Fatalf("expected 503 for fleet NATS down, got %d body %s", rec.Code, rec.String())
		}
	})

	t.Run("context cancel -> unsubscribe sin leak ticker detenido", func(t *testing.T) {
		// Covers [SPEC-002: AC-005/006, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts, sse.WithPingInterval(15*time.Millisecond))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()

		// Act
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case <-done:
			// ok
		case <-time.After(2 * time.Second):
			t.Fatalf("handler did not return after context cancel -> leak") // BR-006
		}
		// dar tiempo a que unsubscribe se ejecute
		time.Sleep(10 * time.Millisecond)

		// Assert
		if !alerts.unsubCalled { // AC-005/006 unsubscribe sin leak
			t.Fatalf("expected unsubscribe called after context cancel, got %v", alerts.unsubCalled)
		}
		// ticker debe detenerse: no más flushes tras cancel
		countAfter := rec.flushCount
		time.Sleep(30 * time.Millisecond)
		if rec.flushCount != countAfter {
			t.Fatalf("expected ticker stopped after cancel, flushCount changed %d -> %d", countAfter, rec.flushCount) // BR-006 leak
		}
	})

	t.Run("fleet context cancel -> unsubscribe", func(t *testing.T) {
		// Covers [SPEC-002: AC-001, BR-006]
		// Arrange
		positions := &fakeTelemetrySubscriber{ch: make(chan sse.PosMsg, 1)}
		h := buildFleetHandler(positions, sse.WithPingInterval(15*time.Millisecond))
		req := httptest.NewRequest(http.MethodGet, "/api/fleet/positions/stream", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()

		// Act
		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("fleet handler leak after cancel") // AC-001
		}

		// Assert
		if !positions.unsubCalled { // AC-001
			t.Fatalf("expected fleet unsubscribe on cancel")
		}
	})
}

// Covers [SPEC-002: AC-005/006/001, BR-006] formato SSE canónico
func TestSSEHandler_FormatAndHeaders(t *testing.T) {
	t.Run("headers SSE correctos Content-Type Cache-Control Connection X-Accel-Buffering", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()
		time.Sleep(20 * time.Millisecond)
		cancel()
		<-done

		// Assert
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") { // BR-006
			t.Fatalf("expected Content-Type text/event-stream, got %q", ct)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(strings.ToLower(cc), "no-cache") {
			t.Fatalf("expected Cache-Control no-cache, got %q", cc) // BR-006
		}
		if conn := rec.Header().Get("Connection"); !strings.Contains(strings.ToLower(conn), "keep-alive") && conn != "" {
			// algunos no setean Connection explícito, toleramos vacío pero documentamos
			t.Logf("Connection header %q", conn)
		}
		if xb := rec.Header().Get("X-Accel-Buffering"); xb != "no" && xb != "" {
			t.Fatalf("expected X-Accel-Buffering no, got %q", xb) // BR-006
		}
		if !strings.Contains(rec.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatalf("missing text/event-stream")
		}
	})

	t.Run("retry:5000 presente en stream", func(t *testing.T) {
		// Covers [SPEC-002: AC-005, BR-006]
		// Arrange
		alerts := &fakeAlertSubscriber{ch: make(chan sse.AlertMsg, 1)}
		h := buildAlertHandler(alerts, sse.WithPingInterval(10*time.Second))
		req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
		req.Header.Set("Accept", "text/event-stream")
		ctx, cancel := context.WithCancel(req.Context())
		req = req.WithContext(ctx)
		rec := newSSERecorder()
		done := make(chan struct{})
		go func() { h.ServeHTTP(rec, req); close(done) }()
		time.Sleep(20 * time.Millisecond)
		cancel()
		<-done
		body := rec.String()

		// Assert
		// BR-006 retry:5000 debe enviarse al inicio o junto a eventos
		if !strings.Contains(body, "retry: 5000") && !strings.Contains(body, "retry:5000") {
			t.Fatalf("expected retry: 5000 in SSE preamble, got %q", body) // BR-006 AC-006
		}
	})
}
